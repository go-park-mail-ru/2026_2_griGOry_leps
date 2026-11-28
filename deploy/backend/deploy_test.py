import os
from pathlib import Path
import subprocess
import tempfile
import unittest


BACKEND = Path(__file__).resolve().parent


class DeploymentTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="onlineconf-deploy-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        (self.root / "releases").mkdir()
        self.log = self.root / "commands.log"
        self.env = dict(os.environ, PATH=f"{self.bin}:{os.environ['PATH']}",
                        TEST_ROOT=str(self.root), TEST_LOG=str(self.log))
        self.executable("systemctl", '''#!/bin/bash
printf '%s\\n' "$*" >> "$TEST_LOG"
case "$1" in
    is-active|is-enabled) exit 1 ;;
esac
if [[ $1 == start || $1 == restart ]]; then
    for service in "${@:2}"; do
        if [[ $service == goget-backend@*.service ]]; then
            name=${service#goget-backend@}
            name=${name%.service}
            [[ -x "$TEST_ROOT/current/$name" ]] || exit 1
        fi
    done
fi
''')
        self.executable("sleep", "#!/bin/bash\nexit 0\n")
        self.executable("curl", '''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
url = sys.argv[-1]
if os.environ.get("FAIL_LEGACY") == "1" and Path(os.environ["TEST_ROOT"], "current").resolve().name == "old":
    sys.exit(22)
with open(os.environ["TEST_LOG"], "a") as log:
    log.write("curl " + url + "\\n")
if "/onlineconf/" in url:
    print(os.environ.get("ADMIN_STATUS", "401"), end="")
elif url.endswith("/api/cart"):
    print("401", end="")
elif "/api/search?" in url:
    print("200", end="")
elif url.endswith("/api/csrf"):
    print(json.dumps({"csrf_token": "valid"}))
elif "/api/listings?" in url:
    print(json.dumps({"items": []}))
elif url.endswith("/api/health"):
    service = {"8081": "auth", "8082": "ads", "8083": "search", "8084": "onlineconf"}[url.split(":")[-1].split("/")[0]]
    print(json.dumps({"status": "ok", "service": service}))
else:
    sys.exit(1)
''')
        self.executable("jq", '''#!/usr/bin/env python3
import json, sys
value = json.load(sys.stdin)
if "--arg" in sys.argv:
    service = sys.argv[sys.argv.index("--arg") + 2]
    ok = value.get("status") == "ok" and value.get("service") == service
elif "csrf_token" in value:
    ok = isinstance(value["csrf_token"], str) and bool(value["csrf_token"])
else:
    ok = isinstance(value.get("items"), list)
sys.exit(0 if ok else 1)
''')
        for name in ("activate.sh", "health.sh"):
            script = (BACKEND / name).read_text().replace("/opt/goget/backend", str(self.root))
            script = script.replace("(( EUID != 0 )) || ", "")
            script = script.replace("/run/lock/goget-backend.lock", str(self.root / "activate.lock"))
            (self.bin / name).write_text(script)
            (self.bin / name).chmod(0o755)

    def executable(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def release(self, name, onlineconf):
        release = self.root / "releases" / name
        release.mkdir()
        services = ["auth", "ads", "search", "gateway"]
        if onlineconf:
            services.append("onlineconf")
        for service in services:
            path = release / service
            path.touch()
            path.chmod(0o755)
        return release

    def run_script(self, name, *args, **extra_env):
        return subprocess.run([str(self.bin / name), *args], env=dict(self.env, **extra_env),
                              text=True, capture_output=True, timeout=15)

    def commands(self):
        return self.log.read_text() if self.log.exists() else ""

    def test_activate_modern_release(self):
        new = self.release("new", onlineconf=True)
        result = self.run_script("activate.sh", "new")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.root / "current").resolve(), new)
        self.assertIn("restart goget-backend@auth.service goget-backend@ads.service goget-backend@search.service goget-backend@onlineconf.service goget-gateway.service", self.commands())
        self.assertIn("curl http://127.0.0.1:8000/api/onlineconf/admin/me", self.commands())

    def test_manual_rollback_to_legacy_release(self):
        old = self.release("old", onlineconf=False)
        new = self.release("new", onlineconf=True)
        (self.root / "current").symlink_to(new)
        result = self.run_script("activate.sh", "--legacy", "old")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.root / "current").resolve(), old)
        self.assertIn("restart goget-backend@auth.service goget-backend@ads.service goget-backend@search.service goget-gateway.service", self.commands())
        self.assertNotIn("curl http://127.0.0.1:8084", self.commands())
        self.assertNotIn("/api/onlineconf/admin/me", self.commands())
        self.assertIn("stop goget-backend@onlineconf.service", self.commands())

    def test_incomplete_modern_release_is_rejected(self):
        self.release("incomplete", onlineconf=False)
        result = self.run_script("activate.sh", "incomplete")
        self.assertEqual(result.returncode, 1)
        self.assertIn("Missing executable:", result.stderr)
        self.assertFalse((self.root / "current").exists())

    def test_failed_first_activation_restores_legacy_release(self):
        old = self.release("old", onlineconf=False)
        self.release("new", onlineconf=True)
        (self.root / "current").symlink_to(old)
        result = self.run_script("activate.sh", "new", ADMIN_STATUS="500")
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertEqual((self.root / "current").resolve(), old)
        self.assertIn("start goget-backend@auth.service goget-backend@ads.service goget-backend@search.service goget-gateway.service", self.commands())
        self.assertNotIn("Rollback requires operator attention", result.stderr)

    def test_failed_legacy_activation_restores_modern_release(self):
        self.release("old", onlineconf=False)
        new = self.release("new", onlineconf=True)
        (self.root / "current").symlink_to(new)
        result = self.run_script("activate.sh", "--legacy", "old", FAIL_LEGACY="1")
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertEqual((self.root / "current").resolve(), new)
        self.assertIn("stop goget-backend@onlineconf.service", self.commands())
        self.assertIn("start goget-backend@auth.service goget-backend@ads.service goget-backend@search.service goget-backend@onlineconf.service goget-gateway.service", self.commands())
        self.assertNotIn("Rollback requires operator attention", result.stderr)

    def test_health_rejects_failed_admin_chain(self):
        result = self.run_script("health.sh", ADMIN_STATUS="500")
        self.assertEqual(result.returncode, 1)
        self.assertIn("gateway -> onlineconf -> auth gRPC: admin check returned 500", result.stderr)

    def test_health_requires_onlineconf_without_release_link(self):
        result = self.run_script("health.sh")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("auth, ads, search, onlineconf and gateway are ready", result.stdout)
        self.assertIn("curl http://127.0.0.1:8084/api/health", self.commands())


if __name__ == "__main__":
    unittest.main()
