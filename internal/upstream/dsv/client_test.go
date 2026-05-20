package dsv_test

import (
    "context"
    "errors"
    "path/filepath"
    "os"
    "runtime"
    "strings"
    "testing"

    "github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
    "github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
    "github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
)

func testdataDir(t *testing.T) string {
    t.Helper()
    _, file, _, ok := runtime.Caller(0)
    if !ok {
        t.Fatal("runtime.Caller failed")
    }
    return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata")
}

func readFixture(t *testing.T, name string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join(testdataDir(t), name))
    if err != nil {
        t.Fatalf("read fixture %s: %v", name, err)
    }
    return data
}

type fakeFetcher struct {
    lastPage string
    lastXhr  string
    resp     []byte
    err      error
}

func (f *fakeFetcher) FetchJSON(ctx context.Context, pageURL, xhrSubstring string) ([]byte, error) {
    f.lastPage = pageURL
    f.lastXhr = xhrSubstring
    if f.err != nil {
        return nil, f.err
    }
    return f.resp, nil
}

func newClientWithFetcher(f dsv.Fetcher) *dsv.Client {
    return dsv.NewClient(dsv.ClientConfig{
        Browser: f,
        Metrics: obs.NewMetrics(),
    })
}

func TestClient_Search_Success(t *testing.T) {
    fixture := readFixture(t, "search_single_result.json")
    ff := &fakeFetcher{resp: fixture}
    c := newClientWithFetcher(ff)
    dto, err := c.Search(context.Background(), "LKG6022524")
    if err != nil {
        t.Fatalf("Search: %v", err)
    }
    if len(dto.Result) != 1 {
        t.Fatalf("expected 1 result, got %d", len(dto.Result))
    }
    if dto.Result[0].Stt != "LKG6022524" {
        t.Fatalf("unexpected STT %q", dto.Result[0].Stt)
    }
    if ff.lastPage == "" || ff.lastXhr == "" {
        t.Fatalf("fetcher not invoked")
    }
}

func TestClient_Detail_Success(t *testing.T) {
    fixture := readFixture(t, "delivered_ltl_se_fr.json")
    ff := &fakeFetcher{resp: fixture}
    c := newClientWithFetcher(ff)
    sid := "LandStt:VAN5022058:CTTS:LAND"
    dto, err := c.Detail(context.Background(), sid)
    if err != nil {
        t.Fatalf("Detail: %v", err)
    }
    if dto.STTNumber != "VAN5022058" {
        t.Fatalf("unexpected STTNumber %q", dto.STTNumber)
    }
    if !strings.Contains(ff.lastXhr, "LandStt:VAN5022058:CTTS:LAND") {
        t.Fatalf("xhr substring incorrect: %s", ff.lastXhr)
    }
}

func TestClient_NotFound_MapsToShipmentNotFound(t *testing.T) {
    ff := &fakeFetcher{err: &domain.UpstreamError{Err: domain.ErrShipmentNotFound}}
    c := newClientWithFetcher(ff)
    _, err := c.Search(context.Background(), "nonexistent")
    if err == nil {
        t.Fatal("expected error")
    }
    if !errors.Is(err, domain.ErrShipmentNotFound) {
        t.Fatalf("expected ErrShipmentNotFound, got %v", err)
    }
}

func TestClient_MalformedJSON_MapsToMalformedResponse(t *testing.T) {
    ff := &fakeFetcher{resp: []byte("not json")}
    c := newClientWithFetcher(ff)
    _, err := c.Search(context.Background(), "any")
    if err == nil {
        t.Fatal("expected error")
    }
    if !errors.Is(err, domain.ErrMalformedResponse) {
        t.Fatalf("expected ErrMalformedResponse, got %v", err)
    }
}
