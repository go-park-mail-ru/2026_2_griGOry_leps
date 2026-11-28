package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/onlineconf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
	"github.com/stretchr/testify/require"
)

type onlineconfTestAdmin struct{}

func (onlineconfTestAdmin) Me(context.Context, string) (domain.User, error) {
	return domain.User{ID: 1}, nil
}

func (onlineconfTestAdmin) CheckAdmin(context.Context, int32) error { return nil }

func TestOnlineconfAPI(t *testing.T) {
	db := newTestPool(t)
	_, err := db.Exec(`INSERT INTO user_account(first_name,email,phone_number,password_hash) VALUES ('Admin','audit@example.test','+79000000000','unused')`)
	require.NoError(t, err)
	service := usecase.NewOnlineconfUsecase(NewOnlineconfRepository(db))
	tokens := csrf.New([]byte("test-secret"), time.Hour)
	admin := onlineconfTestAdmin{}
	router := http.NewServeMux()
	router.Handle("/v1/", delivery.NewOnlineconfRouter(service))
	router.Handle("/", delivery.NewOnlineConfRouter("", delivery.NewSessionMiddleware(admin),
		delivery.NewCSRFHandler(tokens, false), delivery.NewOnlineConfAccessHandler(admin), service))
	request := func(method, path, body string, status int) string {
		t.Helper()
		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "test"})
		req.Header.Set("X-CSRF-Token", tokens.Generate("session:test", time.Now()))
		router.ServeHTTP(res, req)
		require.Equal(t, status, res.Code, res.Body.String())
		return res.Body.String()
	}
	const path = "/api/onlineconf/admin/entries"
	request("POST", "/v1/entries", "{}", 405)
	request("PATCH", "/v1/entries/1", "{}", 404)
	for _, query := range []string{"scope=admin", "scope=%ZZ", "scope=search&scope=admin", "scop=search"} {
		request("GET", path+"?"+query, "", 400)
	}
	const flag = `{"name":"my_flag","scope":"auth","kind":"flag","value":false}`
	request("POST", path, flag, 201)
	request("POST", path, flag, 409)
	request("PATCH", path+"/1", `{"value":true}`, 200)
	request("PATCH", path+"/1", `{"value":null}`, 400)
	var history []domain.OnlineconfChange
	require.NoError(t, json.Unmarshal([]byte(request("GET", path+"/1/history", "", 200)), &history))
	require.Len(t, history, 2)
	require.Equal(t, int32(1), history[0].ActorID)
	require.Equal(t, "false", string(history[0].Before.Value))
	require.Equal(t, "true", string(history[0].After.Value))
	require.Nil(t, history[1].Before)
	request("GET", path+"/999/history", "", 404)
	require.JSONEq(t, `[{"id":1,"name":"my_flag","scope":"auth","kind":"flag","value":true}]`,
		request("GET", path+"?name=Y_F&scope=auth&kind=flag", "", 200))
	created := request("POST", path, `{"name":"cfg","scope":"ads","kind":"config","value":[1,2]}`, 201)
	var entry struct {
		ID    int
		Value json.RawMessage
	}
	require.NoError(t, json.Unmarshal([]byte(created), &entry))
	result := request("PATCH", path+"/"+strconv.Itoa(entry.ID), `{"value":null}`, 200)
	require.NoError(t, json.Unmarshal([]byte(result), &entry))
	require.JSONEq(t, "null", string(entry.Value))
	request("PATCH", path+"/999", `{"value":0}`, 404)
	const rollout = `{"rollout_percent":25,"included_user_ids":[1,2],"excluded_user_ids":[2]}`
	created = request("POST", path, `{"name":"price_sort","scope":"search","kind":"flag","value":`+rollout+`}`, 201)
	require.NoError(t, json.Unmarshal([]byte(created), &entry))
	require.JSONEq(t, rollout, string(entry.Value))
	server := httptest.NewServer(router)
	defer server.Close()
	cache := onlineconf.NewSearchCache(server.URL + "/v1/entries")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		cache.Run(ctx, time.Hour)
	}()
	defer func() { cancel(); <-done }()
	require.Eventually(t, func() bool {
		snapshot := cache.Snapshot()
		return snapshot.PriceSortEnabled(1) && !snapshot.PriceSortEnabled(2)
	}, 3*time.Second, 10*time.Millisecond)
	rolloutPath := path + "/" + strconv.Itoa(entry.ID)
	for _, tc := range []struct {
		id   int
		want string
	}{
		{12, `{"enabled":true}`}, // bucket 9 at 25 percent
		{1, `{"enabled":true}`},  // manually included
		{2, `{"enabled":false}`}, // exclusion wins over inclusion
	} {
		require.JSONEq(t, tc.want, request("GET", rolloutPath+"/check?user_id="+strconv.Itoa(tc.id), "", 200))
	}

	for _, value := range []string{
		`{"rollout_percent":101,"included_user_ids":[],"excluded_user_ids":[]}`,
		`{"rollout_percent":-1,"included_user_ids":[],"excluded_user_ids":[]}`,
		`{"rollout_percent":25,"included_user_ids":["1"],"excluded_user_ids":[]}`,
		`{"rollout_percent":25,"included_user_ids":[]}`,
	} {
		request("PATCH", rolloutPath, `{"value":`+value+`}`, 400)
	}
	for _, percentage := range []string{"0", "100"} {
		request("PATCH", rolloutPath, `{"value":{"rollout_percent":`+percentage+`,"included_user_ids":[],"excluded_user_ids":[]}}`, 200)
	}
	const lengthEntry = `{"scope":"search","kind":"config","name":"max_query_length","value":`
	for _, value := range []string{`"10"`, "0", "-1", "1.5", "null"} {
		request("POST", path, lengthEntry+value+"}", 400)
	}
	created = request("POST", path, lengthEntry+"128}", 201)
	require.NoError(t, json.Unmarshal([]byte(created), &entry))
	request("PATCH", path+"/"+strconv.Itoa(entry.ID), `{"value":"128"}`, 400)
}
