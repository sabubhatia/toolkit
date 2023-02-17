package toolkit

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestToolsRandomString(t *testing.T) {
	var testTools Tools

	n := 10
	s := testTools.RandomString(n)
	if len(s) != n {
		t.Errorf("Unexpected length. Expected: %d, Got: %d", n, len(s))
	}
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{name: "allowed no rename", allowedTypes: []string{"image/png", "image/jpeg"}, renameFile: false, errorExpected: false},
	{name: "allowed rename", allowedTypes: []string{"image/png", "image/jpeg"}, renameFile: true, errorExpected: false},
	{name: "File type not allowed", allowedTypes: []string{"image/png"}, renameFile: true, errorExpected: true},
}

func TestToolsUploadFile(t *testing.T) {
	for _, e := range uploadTests {
		// set up a pipe to avoid buffering
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer func() {
				writer.Close()
			}()
			defer wg.Done()
			part, err := writer.CreateFormFile("file", "./testdata/toby.jpg")
			if err != nil {
				t.Error(err.Error())
				return
			}

			f, err := os.Open("./testdata/toby.jpg")
			if err != nil {
				t.Error(err.Error())
				return
			}
			defer f.Close()
			img, _, err := image.Decode(f)
			if err != nil {
				t.Error("Unable to decode image ", err.Error())
			}
			if err := jpeg.Encode(part, img, nil); err != nil {
				t.Error("Unable to encode image ", err.Error())
			}
		}()
		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Set("content-type", writer.FormDataContentType())
		testTool := Tools{AllowedTypes: e.allowedTypes}
		uploadedFiles, err := testTool.UploadFile(request, "./testdata/upload", e.renameFile)
		if err != nil && !e.errorExpected {
			t.Error(err)
		}

		if !e.errorExpected {
			nf := fmt.Sprintf("./testdata/upload/%s", uploadedFiles[0].NewFileName)
			if _, err := os.Stat(nf); err != nil && err == fs.ErrNotExist {
				t.Errorf("%s, expected the file to exist", e.name)
			}

			// clean up
			os.Remove(nf)
		}

		if e.errorExpected && err == nil {
			t.Errorf("%s expected an error but none exists", e.name)
		}
		wg.Wait()
	}
}

func TestToolsUploadOneFile(t *testing.T) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		defer writer.Close()
		part, err := writer.CreateFormFile("file", "./testdata/toby.jpg")
		if err != nil {
			t.Error(err)
			return
		}
		f, err := os.Open("./testdata/toby.jpg")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			t.Error(err)
			return
		}

		err = jpeg.Encode(part, img, nil)
		if err != nil {
			t.Error(err)
			return
		}
	}()
	r := httptest.NewRequest("POST", "/", pr)
	r.Header.Add("content-type", writer.FormDataContentType())
	testTool := Tools{AllowedTypes: []string{"image/jpeg"}}
	uploadedFile, err := testTool.UploadOneFile(r, "./testdata/upload", true)
	if err != nil {
		t.Error(err)
		return
	}

	nf := fmt.Sprintf("./testdata/upload/%s", uploadedFile.NewFileName)
	_, err = os.Stat(nf)
	if err != nil && err == fs.ErrNotExist {
		t.Error("File Expected to exist")
		return
	}
	os.Remove(nf)
}

func TestToolsCreateDirIfNotExists(t *testing.T) {
	var testTool Tools
	d := "./testdata/mydir"
	os.RemoveAll(d)
	err := testTool.CreateDirIfNotExists(d)
	if err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(d); errors.Is(err, fs.ErrNotExist) {
		t.Error("expected the directory to exist ", err)
	}
	os.RemoveAll(d)
}

var slugTests = []struct {
	name        string
	s           string
	expected    string
	errExpected bool
}{
	{name: "valid string", s: "Now is the time for black", expected: "now-is-the-time-for-black", errExpected: false},
	{name: "empty string", s: "", expected: "", errExpected: true},
	{name: "complex string", s: "NOW IS THE time&$___&$& for><><>&!!black%^%&%*&)123", expected: "now-is-the-time-for-black", errExpected: false},
	{name: "thai string", s: "สวัสดีชาวโลก", expected: "", errExpected: true,},
	{name: "thai string and roman characters", s: "hello blackสวัสดีชาวโลก", expected: "hello-black", errExpected: false},
}

func TestToolsSlugify(t *testing.T) {
	var tools Tools
	for _, e := range slugTests {
		slug, err := tools.Slugify(e.s)
		if err != nil {
			if e.errExpected {
				continue
			}
			if !e.errExpected {
				t.Errorf("%s: error received when none expected %s", e.name, err)
				continue
			}
		}
		if !e.errExpected && !strings.EqualFold(slug, e.expected) {
			t.Errorf("%s after slugify expected %s got %s", e.name, e.expected, slug)
			continue
		}

		if e.errExpected {
			t.Errorf("%s error expected none received", e.name)
			continue
		}
	}
}

func TestToolsDownloadStaticFile(t *testing.T) {
	rr := httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	var testTools Tools
	testTools.DownLoadStaticFile(rr, r, "./testdata", "mypic2002.jpg", "sabu.jpg") 
	res := rr.Result()
	defer res.Body.Close()
	if res.Header[http.CanonicalHeaderKey("content-length")][0] != "259392" {
		t.Errorf("wrong content length of %s", res.Header[http.CanonicalHeaderKey("content-length")][0])
	}
	if res.Header[http.CanonicalHeaderKey("content-disposition")][0] != "attachment; filename=\"sabu.jpg\"" {
		t.Error("wrong content-disposition of ", res.Header[http.CanonicalHeaderKey("content-disposition")][0])
	}

	b, err := io.ReadAll(res.Body)
	if err != nil || len(b) < 1 {
		t.Error("unecpected failure on reading result body")
	}
}

var testJSON = []struct {
	name string
	json string
	errExpected bool
	maxSize int
	allowUnknown bool
} {
	{name: "good json", json:`{"foo": "bar"}`, errExpected: false, maxSize: 1024, allowUnknown: false},
	{name: "badly formatted json", json:`{"foo"}`, errExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "incorrect type", json:`{"foo": 1}`, errExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "two json files", json:`{"foo": "bar"} {"alpha" : "beta"}`, errExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "empty body", json:``, errExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "json syntax error", json:`{"foo": "bar}`, errExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "unknown field", json:`{"fooXYZ": "bar"}`, errExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "allow unknown fields in JSOn", json:`{"foo1": "bar"}`, errExpected: false, maxSize: 1024, allowUnknown: true},
	{name: "missing field name", json:`{jack: "bar"}`, errExpected: true, maxSize: 1024, allowUnknown: true},
	{name: "file too large", json:`{"foo": "bar"}`, errExpected: true, maxSize: 5, allowUnknown: false},
	{name: "not json", json:`Hello, World`, errExpected: true, maxSize: 1024, allowUnknown: false},

}

func TestToolsReadJSON(t *testing.T) {
	var testTool Tools
	for _, e := range testJSON {
		rr := httptest.NewRecorder()
		r, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(e.json)))
		if err != nil {
			t.Errorf("cannot create reader. %s", err.Error())
			continue
		}
		r.Header.Set("content-type", "application/json")
		testTool.MaxJSONSize = e.maxSize
		testTool.AllowUnknownFields = e.allowUnknown
		var decodedJson struct {
			Foo string `json:"foo"`
		}
		err = testTool.ReadJSON(rr, r, &decodedJson)
		if err == nil && e.errExpected {
			t.Errorf("%s: error expected, none received", e.name)
			continue
		}
		if err != nil && !e.errExpected {
			t.Errorf("%s: error was not expected but received error %s", e.name, err.Error())
			continue
		}
		r.Body.Close()

	} 
}

func TestToolsWriteJSON(t *testing.T) {
	var testTools Tools
	payload := JSONResponse{
		Error: false,
		Message: "Bar",
	}

	headers := make(http.Header)
	headers.Add("foo", "bar")
	rr := httptest.NewRecorder()
	err := testTools.WriteJSON(rr, http.StatusOK, payload, headers)
	if err != nil {
		t.Errorf(err.Error())
	}

	// test the headers. Given we have content type set in the method we will get 1 more than we send in
	if len(rr.Result().Header) != len(headers) + 1 {
		t.Errorf("expected length of headers %d, received %d", len(headers) + 1, len(rr.Result().Header))
		return
	}

	// We will range over what we passed in to figure out if everything has come back. What comes
	// back is one more than what we send in. Hence ranging over what we sent should not be expected to throw any 
	// errors
	rhdr := rr.Result().Header
	for k, v := range headers {
		// does the header exist in the results header
		vr, ok := rhdr[k]
		if !ok {
			t.Errorf("header %s expected to exist but not found", k)
			continue
		}
		// is the number of values in the header same as the original
		if len(v) != len(vr) {
			t.Errorf("expected length of value slice to be %d received %d", len(v), len(vr))
			continue
		}
		sort.Strings(vr)
		// Does every value in the original header exist in the result 
		for _, s := range v {
			 i := sort.SearchStrings(vr, s)
			 if i >= len(vr) {
				t.Errorf("value %s expected to exist in the header map for key %s but does not", s, k)
				break
			 }
		}
	}	

	// Now read the returned request and read the json from it into the data structure.
	// This should result in an identical payload.
	body := rr.Result().Body
	r, err := http.NewRequest(http.MethodPost, "/", body) // posts the json 
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	var resultPayload JSONResponse
	rr = httptest.NewRecorder()
	r.Header.Set("content-type", "application/json")
	err = testTools.ReadJSON(rr, r, &resultPayload)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	if payload != resultPayload {
		t.Errorf("expected %+v, Received %+v", payload, resultPayload)
	}
	
}

func TestToolsErrJSON(t * testing.T) {
	var testTool Tools

	errRes := JSONResponse {
		Error: true,
		Message: "This is a test error",
	}
	rr := httptest.NewRecorder()
	statusCode := http.StatusUnauthorized
	contentType := "application/json"
	terr := fmt.Errorf("%s", errRes.Message)
	err := testTool.ErrorJSON(rr, terr, statusCode)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
		return
	}

	if rr.Result().Header.Get("content-type") != contentType {
		t.Errorf("Expected content type %s received %s", contentType, rr.Result().Header.Get("content-type"))
	}

	if rr.Result().StatusCode != statusCode{
		t.Errorf("Expected staus code %d received %d", statusCode, rr.Result().StatusCode)
	}

	r, err := http.NewRequest(http.MethodPost, "/", rr.Result().Body)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	r.Header.Set("content-type", contentType)
	var jErr JSONResponse
	if err := testTool.ReadJSON(httptest.NewRecorder(), r, &jErr); err != nil {
		t.Errorf(err.Error())
		return
	}

	if errRes != jErr {
		t.Errorf("expected %+v received %+v", errRes, jErr)
	}
}
