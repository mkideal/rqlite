package http

import (
	"fmt"
	"net/http"
	"testing"

	sql "github.com/rqlite/rqlite/db"
	"github.com/rqlite/rqlite/store"
)

func Test_NormalizeAddr(t *testing.T) {
	tests := []struct {
		orig string
		norm string
	}{
		{
			orig: "http://localhost:4001",
			norm: "http://localhost:4001",
		},
		{
			orig: "https://localhost:4001",
			norm: "https://localhost:4001",
		},
		{
			orig: "https://localhost:4001/foo",
			norm: "https://localhost:4001/foo",
		},
		{
			orig: "localhost:4001",
			norm: "http://localhost:4001",
		},
		{
			orig: "localhost",
			norm: "http://localhost",
		},
		{
			orig: ":4001",
			norm: "http://:4001",
		},
	}

	for _, tt := range tests {
		if NormalizeAddr(tt.orig) != tt.norm {
			t.Fatalf("%s not normalized correctly, got: %s", tt.orig, tt.norm)
		}
	}
}

func Test_NewService(t *testing.T) {
	m := &MockStore{}
	s := New("127.0.0.1:0", m, nil)
	if s == nil {
		t.Fatalf("failed to create new service")
	}
}

func Test_404Routes(t *testing.T) {
	m := &MockStore{}
	s := New("127.0.0.1:0", m, nil)
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service")
	}
	defer s.Close()
	host := fmt.Sprintf("http://%s", s.Addr().String())

	client := &http.Client{}

	resp, err := client.Get(host + "/db/xxx")
	if err != nil {
		t.Fatalf("failed to make request")
	}
	if resp.StatusCode != 404 {
		t.Fatalf("failed to get expected 404, got %d", resp.StatusCode)
	}

	resp, err = client.Post(host+"/xxx", "", nil)
	if err != nil {
		t.Fatalf("failed to make request")
	}
	if resp.StatusCode != 404 {
		t.Fatalf("failed to get expected 404, got %d", resp.StatusCode)
	}
}

func Test_405Routes(t *testing.T) {
	m := &MockStore{}
	s := New("127.0.0.1:0", m, nil)
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service")
	}
	defer s.Close()
	host := fmt.Sprintf("http://%s", s.Addr().String())

	client := &http.Client{}

	resp, err := client.Get(host + "/db/execute")
	if err != nil {
		t.Fatalf("failed to make request")
	}
	if resp.StatusCode != 405 {
		t.Fatalf("failed to get expected 405, got %d", resp.StatusCode)
	}

	resp, err = client.Post(host+"/db/backup", "", nil)
	if err != nil {
		t.Fatalf("failed to make request")
	}
	if resp.StatusCode != 405 {
		t.Fatalf("failed to get expected 405, got %d", resp.StatusCode)
	}

	resp, err = client.Post(host+"/status", "", nil)
	if err != nil {
		t.Fatalf("failed to make request")
	}
	if resp.StatusCode != 405 {
		t.Fatalf("failed to get expected 405, got %d", resp.StatusCode)
	}
}

func Test_401Routes_NoBasicAuth(t *testing.T) {
	c := &mockCredentialStore{CheckOK: false, HasPermOK: false}

	m := &MockStore{}
	s := New("127.0.0.1:0", m, c)
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service")
	}
	defer s.Close()
	host := fmt.Sprintf("http://%s", s.Addr().String())

	client := &http.Client{}

	for _, path := range []string{
		"/db/execute",
		"/db/query",
		"/db/backup",
		"/join",
		"/status",
	} {
		resp, err := client.Get(host + path)
		if err != nil {
			t.Fatalf("failed to make request")
		}
		if resp.StatusCode != 401 {
			t.Fatalf("failed to get expected 401 for path %s, got %d", path, resp.StatusCode)
		}
	}
}

func Test_401Routes_BasicAuthBadPassword(t *testing.T) {
	c := &mockCredentialStore{CheckOK: false, HasPermOK: false}

	m := &MockStore{}
	s := New("127.0.0.1:0", m, c)
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service")
	}
	defer s.Close()
	host := fmt.Sprintf("http://%s", s.Addr().String())

	client := &http.Client{}

	for _, path := range []string{
		"/db/execute",
		"/db/query",
		"/db/backup",
		"/join",
		"/status",
	} {
		req, err := http.NewRequest("GET", host+path, nil)
		if err != nil {
			t.Fatalf("failed to create request: %s", err.Error())
		}
		req.SetBasicAuth("username1", "password1")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed to make request: %s", err.Error())
		}
		if resp.StatusCode != 401 {
			t.Fatalf("failed to get expected 401 for path %s, got %d", path, resp.StatusCode)
		}
	}
}

func Test_401Routes_BasicAuthBadPerm(t *testing.T) {
	c := &mockCredentialStore{CheckOK: true, HasPermOK: false}

	m := &MockStore{}
	s := New("127.0.0.1:0", m, c)
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service")
	}
	defer s.Close()
	host := fmt.Sprintf("http://%s", s.Addr().String())

	client := &http.Client{}

	for _, path := range []string{
		"/db/execute",
		"/db/query",
		"/db/backup",
		"/join",
		"/status",
	} {
		req, err := http.NewRequest("GET", host+path, nil)
		if err != nil {
			t.Fatalf("failed to create request: %s", err.Error())
		}
		req.SetBasicAuth("username1", "password1")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed to make request: %s", err.Error())
		}
		if resp.StatusCode != 401 {
			t.Fatalf("failed to get expected 401 for path %s, got %d", path, resp.StatusCode)
		}
	}
}

type MockStore struct {
	executeFn func(queries []string, tx bool) ([]*sql.Result, error)
	queryFn   func(queries []string, tx, leader, verify bool) ([]*sql.Rows, error)
}

func (m *MockStore) Execute(queries []string, timings, tx bool) ([]*sql.Result, error) {
	if m.executeFn == nil {
		return nil, nil
	}
	return nil, nil
}

func (m *MockStore) Query(queries []string, timings, tx bool, lvl store.ConsistencyLevel) ([]*sql.Rows, error) {
	if m.queryFn == nil {
		return nil, nil
	}
	return nil, nil
}

func (m *MockStore) Join(addr string) error {
	return nil
}

func (m *MockStore) Leader() string {
	return ""
}

func (m *MockStore) Peer(addr string) string {
	return ""
}

func (m *MockStore) Stats() (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockStore) Backup(leader bool) ([]byte, error) {
	return nil, nil
}

type mockCredentialStore struct {
	CheckOK   bool
	HasPermOK bool
}

func (m *mockCredentialStore) Check(username, password string) bool {
	return m.CheckOK
}

func (m *mockCredentialStore) HasPerm(username, perm string) bool {
	return m.HasPermOK
}
