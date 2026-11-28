package repository

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
	"github.com/stretchr/testify/require"
)

func TestOnlineconfAPI(t *testing.T) {
	router := delivery.NewOnlineconfRouter(usecase.NewOnlineconfUsecase(NewOnlineconfRepository(newTestPool(t))))
	request := func(method, path, body string, status int) string {
		t.Helper()
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader(body)))
		require.Equal(t, status, res.Code, res.Body.String())
		return res.Body.String()
	}
	const path = "/v1/entries"
	for _, query := range []string{"scope=admin", "scope=%ZZ", "scope=search&scope=admin", "scop=search"} {
		request("GET", path+"?"+query, "", 400)
	}
	const flag = `{"name":"my_flag","scope":"auth","kind":"flag","value":false}`
	request("POST", path, flag, 201)
	request("POST", path, flag, 409)
	request("PATCH", path+"/1", `{"value":true}`, 200)
	request("PATCH", path+"/1", `{"value":null}`, 400)
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
	const rollout = `{"percentage":25,"include_user_ids":[1,2],"exclude_user_ids":[2]}`
	created = request("POST", path, `{"name":"rollout","scope":"ads","kind":"flag","value":`+rollout+`}`, 201)
	require.NoError(t, json.Unmarshal([]byte(created), &entry))
	require.JSONEq(t, rollout, string(entry.Value))
	rolloutPath := path + "/" + strconv.Itoa(entry.ID)
	for _, value := range []string{
		`{"percentage":101,"include_user_ids":[],"exclude_user_ids":[]}`,
		`{"percentage":-1,"include_user_ids":[],"exclude_user_ids":[]}`,
		`{"percentage":25,"include_user_ids":["1"],"exclude_user_ids":[]}`,
		`{"percentage":25,"include_user_ids":[]}`,
	} {
		request("PATCH", rolloutPath, `{"value":`+value+`}`, 400)
	}
	for _, percentage := range []string{"0", "100"} {
		request("PATCH", rolloutPath, `{"value":{"percentage":`+percentage+`,"include_user_ids":[],"exclude_user_ids":[]}}`, 200)
	}
	const lengthEntry = `{"scope":"search","kind":"config","name":"max_query_length","value":`
	for _, value := range []string{`"10"`, "0", "-1", "1.5", "null"} {
		request("POST", path, lengthEntry+value+"}", 400)
	}
	created = request("POST", path, lengthEntry+"128}", 201)
	require.NoError(t, json.Unmarshal([]byte(created), &entry))
	request("PATCH", path+"/"+strconv.Itoa(entry.ID), `{"value":"128"}`, 400)
}
