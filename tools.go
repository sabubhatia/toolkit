package toolkit

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+"

// Tools is the type used to instantiate this module. Any variable of this type will have access to all the methods
// with the receiver *Tools
type Tools struct {
	MaxFileSize        int
	AllowedTypes       []string
	MaxJSONSize        int
	AllowUnknownFields bool
}

// RandomString returns a string of randomn characters of length n, using randomStringSource
// as the source for teh characters of the string
func (t *Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)

	for i := range s {
		p, err := rand.Prime(rand.Reader, len(r))
		if err != nil {
			log.Fatal("Unexpected error ", err)
		}
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}

// UploadedFile is a struct to store information about an file that has been uploaded.
type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

// UploadOneFile is a convenience function that is used to upload just one single file. This simply calls the more
// general UploadFile() function.
func (t *Tools) UploadOneFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	uploadedFile, err := t.UploadFile(r, uploadDir, renameFile)
	if err != nil {
		return nil, err
	}

	return uploadedFile[0], nil
}

// UploadFile reads and loads files to a specified directory. If rename is true it uses the RandomString()
// function to generate a new file name. The extension of the file is always the same as that of the original file name.
func (t *Tools) UploadFile(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	var upLoadedFiles []*UploadedFile

	if t.MaxFileSize == 0 {
		t.MaxFileSize = 1024 * 1024 * 1024 // 1 gb approximately
	}
	if err := r.ParseMultipartForm(int64(t.MaxFileSize)); err != nil {
		log.Println(r)
		log.Fatal("Fatal:", err)
		return nil, errors.New(err.Error())
	}

	if err := t.CreateDirIfNotExists(uploadDir); err != nil {
		return nil, err
	}

	for _, fHeaders := range r.MultipartForm.File {
		for _, hdrs := range fHeaders {
			var err error
			upLoadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
				inFile, err := hdrs.Open()
				if err != nil {
					return nil, err
				}
				defer inFile.Close()

				buf := make([]byte, 512)
				_, err = inFile.Read(buf)
				if err != nil {
					return nil, err
				}

				// Check to see of the file type is permitted.
				allowed := false
				fileType := http.DetectContentType(buf)

				if len(t.AllowedTypes) > 0 {
					for _, x := range t.AllowedTypes {
						if strings.EqualFold(x, fileType) {
							allowed = true
							break
						}
					}
				} else {
					allowed = true
				}
				if !allowed {
					return nil, errors.New("the uploaded file type is not permitted")
				}

				_, err = inFile.Seek(0, 0)
				if err != nil {
					return nil, err
				}
				var uploadedFile UploadedFile
				if renameFile {
					uploadedFile.NewFileName = fmt.Sprintf("%s%s", t.RandomString(25), filepath.Ext(hdrs.Filename))
				} else {
					uploadedFile.NewFileName = hdrs.Filename
				}
				uploadedFile.OriginalFileName = hdrs.Filename

				if outFile, err := os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
					return nil, err
				} else {
					fileSize, err := io.Copy(outFile, inFile)
					if err != nil {
						return nil, err
					}
					uploadedFile.FileSize = fileSize
				}
				uploadedFiles = append(uploadedFiles, &uploadedFile)
				return uploadedFiles, nil

			}(upLoadedFiles)
			if err != nil {
				return upLoadedFiles, err
			}
		}
	}

	return upLoadedFiles, nil
}

// CreateDirIfNotExists creates a directory along with all needed parent directories if they dont exist.
// Function does noting if the directory alreay exists.
func (t *Tools) CreateDirIfNotExists(path string) error {
	if len(strings.TrimSpace(path)) < 1 {
		return errors.New("invalid path")
	}

	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		const mode = 0755
		err := os.MkdirAll(path, mode)
		if err != nil {
			return err
		}
	}

	return nil
}

// Slugify is a very simple means of creating a slug from a string.
func (t *Tools) Slugify(s string) (string, error) {
	if len(s) < 1 {
		return "", errors.New("empty strings are not permitted")
	}

	re, err := regexp.Compile(`[^a-z/d]+`) // allow any lower case alphabets or digits
	if err != nil {
		return "", errors.New(("fatal server error : " + err.Error()))
	}

	slug := strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(slug) <= 0 {
		return "", errors.New("after removing characters, slug is of zero length")
	}

	return slug, nil
}

// DowloadStaticFile downloads a file and tries to force the browser not to display it by setting the
// content-disposition. It also allows specification of the display name.
func (t *Tools) DownLoadStaticFile(w http.ResponseWriter, r *http.Request, p, file, displayName string) {
	f := path.Join(p, file)
	w.Header().Set("content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))
	http.ServeFile(w, r, f)
}

// JSONResponse is the type used for sending JSON around
type JSONResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

//ReadJSON tries to read the JSON from the request and copies it to the provided arbitary data structure
func (t *Tools) ReadJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
	maxJSONSize := 1024 * 1024 // 1 megabye apprxomiately
	if t.MaxJSONSize > 0 {
		maxJSONSize = t.MaxJSONSize
	}

	if r.Header.Get("content-type") != "application/json" {
		return fmt.Errorf(`unexpected content type of "%s"`, r.Header.Get("content-type"))
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxJSONSize))

	dec := json.NewDecoder(r.Body)
	if !t.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}

	err := dec.Decode(data)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshallTypeError *json.UnmarshalTypeError
		var invalidUnmarshallError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly formed JSON (at character %q) ", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return fmt.Errorf("body contains badly formed JSON")
		case errors.As(err, &unmarshallTypeError):
			if unmarshallTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field :%q", unmarshallTypeError.Field)
			}
			return fmt.Errorf("body contains invalid JSON at %d", unmarshallTypeError.Offset)
		case errors.Is(err, io.EOF):
			return fmt.Errorf("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			field := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contains unknown field %q", field)
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxJSONSize)
		case errors.As(err, &invalidUnmarshallError):
			return fmt.Errorf("error unnmarshalling JSON %s", err.Error())
		default:
			return err
		}
	}

	// I Dont want the input containing more than one JSON.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must contain only one JSON value")
	}

	return nil
}


// WriteJSON takes a status code and an arbitary data which it writes out to the client
func (t * Tools) WriteJSON(w http.ResponseWriter, status int, data interface{}, headers...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for k, vs := range headers[0] {
			for  _, v := range vs {
				w.Header().Set(k, v)
			}
		}
	}

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(out); err != nil {
		return err
	}

	return nil
}

// ErroJSON takes an error and an optional status code and writes the error in JSON format as the response.
// The default status if none specified is http.StatusBadRequest
func (t *Tools) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	// create a JSONResponse
	jr := JSONResponse{
		Error: true,
		Message: err.Error(),

	}
	statusCode := http.StatusBadRequest
	if len(status) > 0 {
		statusCode = status[0]
	}

	return t.WriteJSON(w, statusCode, jr)
}

// PushJSONToRemote posts arbitary data to some url as json and returns the response, status code and error 
// if any. The final parameter client is optional. If none is specified we use the standard http.Client
func (t *Tools) PushJSONToRemote(uri string, data interface{}, client ...*http.Client) (*http.Response, int, error) {
	// create json
	jd, err := json.Marshal(data)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	httpClient := &http.Client{}
	if len(client) > 0 {
		httpClient = client[0]
	}

	req, err := http.NewRequest(http.MethodPost, uri, bytes.NewBuffer(jd))
    if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("content-type", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	
	return resp, resp.StatusCode, nil


}
