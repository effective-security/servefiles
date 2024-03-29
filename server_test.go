package servefiles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type serverTestSuite struct {
	suite.Suite
	ft *fakeT
	s  *Server
}

func Test_Server(t *testing.T) {
	suite.Run(t, new(serverTestSuite))
}

func (s *serverTestSuite) SetupTest() {
	s.ft = new(fakeT)
	s.s = New(s.ft, "testdata/primary", "testdata/base")
	s.s.OAuthFixupURL = []string{"/services/oauth2/token"}
}

func (s *serverTestSuite) TearDownTest() {
	s.s.Close()
}

func (s *serverTestSuite) doHTTPCall(method, path string, body io.Reader, expStatusCode int) ([]byte, map[string][]string) {
	req, err := http.NewRequest(method, s.s.URL()+path, body)
	s.Require().NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	s.Require().NoError(err)
	s.Require().Equal(expStatusCode, res.StatusCode)
	return resBody, res.Header
}

func (s *serverTestSuite) Test_Default() {
	resp, _ := s.doHTTPCall(http.MethodGet, "/def", nil, http.StatusOK)
	s.JSONEq(`{"def":true}`, string(resp))
	s.Equal(1, s.s.RequestCount("/def"))
	s.Equal(map[string]int{"/def": 1}, s.s.RequestCounts())
}

func (s *serverTestSuite) Test_WithStatusCode() {
	resp, _ := s.doHTTPCall(http.MethodGet, "/withCode", nil, http.StatusBadRequest)
	s.JSONEq(`{"code":"BOOM"}`, string(resp))
	s.Equal(1, s.s.RequestCount("/withCode"))
	s.Equal(map[string]int{"/withCode": 1}, s.s.RequestCounts())
}

func (s *serverTestSuite) Test_Sequence() {
	s.testSequence("/withSeq", http.StatusOK)
}

func (s *serverTestSuite) Test_SequenceWithCode() {
	s.testSequence("/withSeqAndCode", http.StatusCreated)
}

func (s *serverTestSuite) testSequence(reqPath string, expStatusCode int) {
	resp, _ := s.doHTTPCall(http.MethodGet, reqPath, nil, expStatusCode)
	s.JSONEq(`{"seq":1}`, string(resp))
	s.Equal(1, s.s.RequestCount(reqPath))
	resp, _ = s.doHTTPCall(http.MethodGet, reqPath, nil, expStatusCode)
	s.JSONEq(`{"seq":2}`, string(resp))
	s.Equal(2, s.s.RequestCount(reqPath))
	s.Equal(map[string]int{reqPath: 2}, s.s.RequestCounts())
	// run off the end of supplied sequences, get a 404
	_, _ = s.doHTTPCall(http.MethodGet, reqPath, nil, http.StatusNotFound)
}

func (s *serverTestSuite) Test_Token() {
	resp, _ := s.doHTTPCall(http.MethodGet, "/services/oauth2/token", nil, http.StatusOK)
	var d map[string]interface{}
	s.Require().NoError(json.Unmarshal(resp, &d))
	s.Equal(s.s.URL(), d["instance_url"])
	s.Equal(s.s.URL()+"/id/00DT0000000DpvcMAC/005B0000001JwvAIAS", d["id"])
	s.EqualValues(1459975184111, d["issued_at"])
}

func (s *serverTestSuite) Test_NotAuthToken() {
	// verify that something that looks like the auth token, but is at a different
	// url is not modified.
	resp, _ := s.doHTTPCall(http.MethodGet, "/services/not_oauth/token", nil, http.StatusOK)
	var d map[string]interface{}
	s.Require().NoError(json.Unmarshal(resp, &d))
	s.Equal("https://login.acme.com/id/00DT0000000DpvcMAC/005B0000001JwvAIAS", d["id"])
	s.Equal("https://na1.acme.com/", d["instance_url"])
	s.EqualValues(1459975184111, d["issued_at"])
}

func (s *serverTestSuite) Test_LastRequestBody() {
	reqBody := `{"hello":"world"}`
	resp, _ := s.doHTTPCall(http.MethodPost, "/def", bytes.NewBufferString(reqBody), http.StatusOK)
	s.Equal(reqBody, string(s.s.LastBody("/def")))
	s.JSONEq(`{"def":true}`, string(resp))
	s.Equal(1, s.s.RequestCount("/def"))

	resp, _ = s.doHTTPCall(http.MethodPost, "/def?q=1", bytes.NewBufferString(reqBody), http.StatusOK)
	s.Equal(reqBody, string(s.s.LastBody("/def?q=1")))
	s.JSONEq(`{"def":true,"query":true}`, string(resp))
	s.Equal(1, s.s.RequestCount("/def?q=1"))

	reqBody = `{"hello":"world2"}`
	resp, _ = s.doHTTPCall(http.MethodPut, "/def", bytes.NewBufferString(reqBody), http.StatusOK)
	s.Equal(reqBody, string(s.s.LastBody("/def")))
	s.JSONEq(`{"def":true}`, string(resp))
	s.Equal(2, s.s.RequestCount("/def"))
}

func (s *serverTestSuite) Test_PostFiles() {
	reqBody := `{"type":2}`
	resp, _ := s.doHTTPCall(http.MethodPost, "/v1/post", bytes.NewBufferString(reqBody), http.StatusOK)
	s.JSONEq(`{"response": "2","type": 2}`, string(resp))
	s.Equal(reqBody, string(s.s.LastBody("/v1/post")))
	s.Equal(1, s.s.RequestCount("/v1/post"))

	reqBody = `{"type":1}`
	resp, _ = s.doHTTPCall(http.MethodPost, "/v1/post", bytes.NewBufferString(reqBody), http.StatusOK)
	s.JSONEq(`{"response": "1","type": 1}`, string(resp))
	s.Equal(reqBody, string(s.s.LastBody("/v1/post")))
	s.Equal(2, s.s.RequestCount("/v1/post"))
}

func (s *serverTestSuite) Test_Missing() {
	resp, _ := s.doHTTPCall(http.MethodGet, "/missing", nil, http.StatusNotFound)
	s.JSONEq(`{"code": "NOT_FOUND", "message": "the requested resource does not exist"}`, string(resp))
}

func (s *serverTestSuite) Test_SequencedStatusCodes() {
	s.doHTTPCall(http.MethodGet, "/statusCodes", nil, http.StatusBadRequest)
	s.doHTTPCall(http.MethodGet, "/statusCodes", nil, http.StatusNotFound)
	s.doHTTPCall(http.MethodGet, "/statusCodes", nil, http.StatusOK)
}

func (s *serverTestSuite) Test_VerbPrefix() {
	resp, _ := s.doHTTPCall(http.MethodGet, "/v1/verb", nil, http.StatusOK)
	s.JSONEq(`{"verb":"get"}`, string(resp))
	resp, _ = s.doHTTPCall(http.MethodDelete, "/v1/verb", nil, http.StatusOK)
	s.JSONEq(`{"verb":"delete"}`, string(resp))
	resp, _ = s.doHTTPCall(http.MethodPut, "/v1/verb", nil, http.StatusOK)
	s.JSONEq(`{"verb":"any"}`, string(resp))
}

func (s *serverTestSuite) Test_ContentType() {
	resp, headers := s.doHTTPCall(http.MethodGet, "/v1/ct?ct=text", nil, http.StatusOK)
	s.Equal(`text.plain`, string(resp))
	s.Equal("text/plain", headers["Custom"][0])

	resp, headers = s.doHTTPCall(http.MethodGet, "/v1/ct?ct=tsq", nil, http.StatusOK)
	s.Equal(`application/timestamp-query`, string(resp))
	s.Equal("application/timestamp-query", headers["Custom"][0])

	resp, headers = s.doHTTPCall(http.MethodGet, "/v1/ct?ct=tsr", nil, http.StatusOK)
	s.Equal(`application/timestamp-reply`, string(resp))
	s.Equal("application/timestamp-reply", headers["Custom"][0])

	resp, headers = s.doHTTPCall(http.MethodPut, "/v1/ct", nil, http.StatusOK)
	s.JSONEq(`{"ct":"application/json"}`, string(resp))
	s.Equal("application/json", headers["Custom"][0])

	h := s.s.LastReqHdr("/v1/ct?ct=text")
	s.True(len(h) > 0)
	h = s.s.LastReqHdr("/v1/ct?ct=tsq")
	s.True(len(h) > 0)
	h = s.s.LastReqHdr("/v1/ct?ct=tsr")
	s.True(len(h) > 0)
	h = s.s.LastReqHdr("/v1/ct")
	s.True(len(h) > 0)
}

func Test_StatusCode(t *testing.T) {
	r := requestSettings{}
	assert.Equal(t, 200, r.statusCode(1))
	r.StatusCode = 200
	assert.Equal(t, 200, r.statusCode(1))
	r.StatusCode = 404
	assert.Equal(t, 404, r.statusCode(1))
	r = requestSettings{
		StatusCodes: []int{400, 500, 201},
	}
	assert.Equal(t, 400, r.statusCode(1))
	assert.Equal(t, 500, r.statusCode(2))
	assert.Equal(t, 201, r.statusCode(3))
	assert.Equal(t, 200, r.statusCode(4))
}

func Test_HandleAuthFixup(t *testing.T) {
	src := `{"instance_url": "https://login.acme.com", "id":"https://na1.acme.com/1/2", "sig":1234}`
	dest := &bytes.Buffer{}
	handleAuthFixup("http://127.0.0.1:1234", dest, bytes.NewBufferString(src))
	exp := `{"instance_url": "http://127.0.0.1:1234", "id":"http://127.0.0.1:1234/1/2", "sig":1234}`
	assert.JSONEq(t, dest.String(), exp)
}

func Test_NotFound(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/foo", nil)
	require.NoError(t, err)
	res := httptest.NewRecorder()
	ft := new(fakeT)
	s := Server{t: ft}
	s.notFound(res, req)
	assert.Equal(t, http.StatusNotFound, res.Code)
	assert.JSONEq(t, `{"code": "NOT_FOUND", "message": "the requested resource does not exist"}`, res.Body.String())
	if assert.True(t, len(ft.messages) > 0) {
		assert.Equal(t, ft.messages[0], "no response file exists for /foo, returning a 404 response")
	}
}

// a MinimalTestingT impl that captures info about calls to it, outside of our tests actual testing.T
type fakeT struct {
	messages []string
	failed   bool
}

func (f *fakeT) Logf(format string, args ...interface{}) {
	f.messages = append(f.messages, fmt.Sprintf(format, args...))
}

func (f *fakeT) Errorf(format string, args ...interface{}) {
	f.Logf(format, args...)
	f.failed = true
}

func (f *fakeT) FailNow() {
	f.failed = true
}
