package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/uid"
	"github.com/cockroachdb/pebble"
	"github.com/julienschmidt/httprouter"
)

func main() {
	pPort := fla9.Int("port,p", 8080, "Listen port")
	pImpl := fla9.String("impl", "pebble", "pebble,p/lotusdb,l/pogreb,g")
	fla9.Parse()

	var db DB
	var path string

	switch *pImpl {
	case "pogreb", "g":
		db = &pogrebDB{}
		path = "pogreb"
	case "pebble", "p":
		db = &pebbleDB{}
		path = "pebble"
	case "lotusdb", "l":
		db = &lotusdbDB{}
		path = "lotusdb"
	default:
		log.Fatalf("unknown impl %s", *pImpl)
	}
	if err := db.Open("docdb." + path); err != nil {
		log.Fatalf("open docdb.%s failed: %v", path, err)
	}

	defer iox.Close(db)

	s := &server{DB: db, flushNotify: make(chan struct{})}

	router := httprouter.New()
	router.POST("/docs", wrapHandler(s.addDoc))
	router.GET("/docs", wrapHandler(s.searchDocs))
	router.GET("/docs/:id", wrapHandler(s.getDoc))

	log.Printf("Listening on %d", *pPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *pPort), router))
}

// H is alias for map[string]any.
type H map[string]any

func jsonResponseError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)

	if err := json.NewEncoder(w).Encode(H{"status": "error", "error": err.Error()}); err != nil {
		log.Printf("encode json response failed: %v", err)
	}
}

func jsonResponse(w http.ResponseWriter, body H) error {
	if err := json.NewEncoder(w).Encode(H{"body": body, "status": "ok"}); err != nil {
		log.Printf("encode json response failed: %v", err)
	}
	return nil
}

type DB interface {
	io.Closer

	Open(path string) error
	GetIndex(key []byte) ([]byte, io.Closer, error)
	SetIndex(key, val []byte) error
	GetVal(key []byte) ([]byte, io.Closer, error)
	SetVal(key, val []byte) error
	Walk(walker func(key, val []byte) error) error
	Flush()
}

type server struct {
	DB
	flushNotify chan struct{}
}

// Ignores arrays
func getPathValues(obj H, prefix string) (pvs []string) {
	for key, val := range obj {
		switch t := val.(type) {
		case H:
			pvs = append(pvs, getPathValues(t, key)...)
			continue
		case []interface{}: // Can't handle arrays
			continue
		}

		if prefix != "" {
			key = prefix + "." + key
		}

		pvs = append(pvs, fmt.Sprintf("%s=%v", key, val))
	}

	return pvs
}

func (s server) index(id string, doc H, reindex bool) {
	pv := getPathValues(doc, "")

	for _, pathValue := range pv {
		idsString, closer, err := s.GetIndex([]byte(pathValue))
		if err != nil && err != pebble.ErrNotFound {
			log.Printf("Could not look up pathvalue %s in [%#v]: %v", pathValue, doc, err)
		}

		if len(idsString) == 0 {
			idsString = []byte(id)
		} else if reindex {
			ids := strings.Split(string(idsString), ",")
			idsString = append([]byte{}, idsString...) // recreate idsString
			if found := ss.AnyOf(id, ids...); !found {
				idsString = append(idsString, []byte(","+id)...)
			}
		} else {
			idsString = append([]byte{}, idsString...) // recreate idsString
			idsString = append(idsString, []byte(","+id)...)
		}

		iox.Close(closer)
		if err = s.SetIndex([]byte(pathValue), idsString); err != nil {
			log.Printf("Could not update index: %s", err)
		}
	}
}

func (s *server) addDoc(w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	var doc H
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		return err
	}

	id := uid.New().String() // New ksuid for the document
	s.index(id, doc, false)

	bs, _ := json.Marshal(doc)
	if err := s.SetVal([]byte(id), bs); err != nil {
		return err
	}
	s.notifyFlush()

	return jsonResponse(w, H{"id": id})
}

type queryComparison struct {
	key   []string
	value string
	op    string
}

type query struct {
	ands []queryComparison
}

func getPath(doc H, parts []string) (any, bool) {
	var docSegment any = doc
	for _, part := range parts {
		m, ok := docSegment.(H)
		if !ok {
			return nil, false
		}

		if docSegment, ok = m[part]; !ok {
			return nil, false
		}
	}

	return docSegment, true
}

func (q query) match(doc H) bool {
	for _, arg := range q.ands {
		value, ok := getPath(doc, arg.key)
		if !ok {
			return false
		}

		// Handle equality
		if arg.op == "=" {
			if match := fmt.Sprintf("%v", value) == arg.value; !match {
				return false
			}

			continue
		}

		// Handle <, >
		right, err := strconv.ParseFloat(arg.value, 64)
		if err != nil {
			return false
		}

		var left float64
		switch t := value.(type) {
		case float32, float64:
			left = reflect.ValueOf(value).Float()
		case uint, uint8, uint16, uint32, uint64:
			left = float64(reflect.ValueOf(value).Uint())
		case int, int8, int16, int32, int64:
			left = float64(reflect.ValueOf(value).Int())
		case string:
			if left, err = strconv.ParseFloat(t, 64); err != nil {
				return false
			}
		default:
			return false
		}

		if arg.op == ">" {
			if left <= right {
				return false
			}

			continue
		}

		if left >= right {
			return false
		}
	}

	return true
}

// Handles either quoted strings or unquoted strings of only contiguous digits and letters
func lexString(input []rune, index int) (string, int, error) {
	if index >= len(input) {
		return "", index, nil
	}
	if input[index] == '"' {
		index++
		foundEnd := false

		var s []rune
		// TODO: handle nested quotes
		for index < len(input) {
			if input[index] == '"' {
				foundEnd = true
				break
			}

			s = append(s, input[index])
			index++
		}

		if !foundEnd {
			return "", index, fmt.Errorf("expected end of quoted string")
		}

		return string(s), index + 1, nil
	}

	// If unquoted, read as much contiguous digits/letters as there are
	var s []rune
	// TODO: someone needs to validate there's not ...
	for index < len(input) {
		c := input[index]
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || c == '.') {
			break
		}
		s = append(s, c)
		index++
	}

	if len(s) == 0 {
		return "", index, fmt.Errorf("no string found")
	}

	return string(s), index, nil
}

// E.g. q=a.b:12
func parseQuery(q string) (*query, error) {
	if q == "" {
		return &query{}, nil
	}

	var parsed query
	qRune := []rune(q)
	for i := 0; i < len(qRune); {
		for unicode.IsSpace(qRune[i]) { // Eat whitespace
			i++
		}

		key, nextIndex, err := lexString(qRune, i)
		if err != nil {
			return nil, fmt.Errorf("expected valid key, got [%s]: `%s`", err, q[nextIndex:])
		}

		if q[nextIndex] != ':' {
			return nil, fmt.Errorf("expected colon at %d, got: `%s`", nextIndex, q[nextIndex:])
		}
		i = nextIndex + 1

		op := "="
		if q[i] == '>' || q[i] == '<' {
			op = string(q[i])
			i++
		}

		value, nextIndex, err := lexString(qRune, i)
		if err != nil {
			return nil, fmt.Errorf("expected valid value, got [%s]: `%s`", err, q[nextIndex:])
		}
		i = nextIndex

		arg := queryComparison{key: strings.Split(key, "."), value: value, op: op}
		parsed.ands = append(parsed.ands, arg)
	}

	return &parsed, nil
}

func (s server) getDocumentByID(id []byte) (H, error) {
	valBytes, closer, err := s.GetVal(id)
	defer iox.Close(closer)
	if err != nil {
		return nil, err
	}

	return UnmarshalJSON(valBytes)
}

func UnmarshalJSON(valBytes []byte) (doc H, err error) {
	err = json.Unmarshal(valBytes, &doc)
	return
}

func (s server) lookup(pathValue string) ([]string, error) {
	idsString, closer, err := s.GetIndex([]byte(pathValue))
	if err != nil && err != pebble.ErrNotFound {
		return nil, fmt.Errorf("could not look up pathvalue [%#v]: %s", pathValue, err)
	}
	defer iox.Close(closer)

	return ss.Split(string(idsString), ss.WithSeps(","), ss.WithIgnoreEmpty(true), ss.WithIgnoreEmpty(true)), nil
}

func wrapHandler(h func(http.ResponseWriter, *http.Request, httprouter.Params) error) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := h(w, r, p); err != nil {
			jsonResponseError(w, err)
		}
	}
}
func (s server) searchDocs(w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	q, err := parseQuery(r.URL.Query().Get("q"))
	if err != nil {
		return err
	}

	isRange := false
	idsArgCount := map[string]int{}
	nonRangeArgs := 0
	for _, arg := range q.ands {
		if arg.op == "=" {
			nonRangeArgs++

			ids, err := s.lookup(fmt.Sprintf("%s=%v", strings.Join(arg.key, "."), arg.value))
			if err != nil {
				return err
			}

			for _, id := range ids {
				idsArgCount[id]++
			}
		} else {
			isRange = true
		}
	}

	var idsInAll []string
	for id, count := range idsArgCount {
		if count == nonRangeArgs {
			idsInAll = append(idsInAll, id)
		}
	}

	var docs []any
	if r.URL.Query().Get("skipIndex") == "true" {
		idsInAll = nil
	}
	if len(idsInAll) > 0 {
		for _, id := range idsInAll {
			doc, err := s.getDocumentByID([]byte(id))
			if err != nil {
				return err
			} else if !isRange || q.match(doc) {
				docs = append(docs, H{"id": id, "body": doc})
			}
		}
	} else {
		if err := s.Walk(func(key, val []byte) error {
			if doc, err := UnmarshalJSON(val); err == nil && q.match(doc) {
				docs = append(docs, H{"id": string(key), "body": doc})
			}
			return err
		}); err != nil {
			return err
		}
	}

	return jsonResponse(w, H{"documents": docs, "count": len(docs)})
}

func (s server) getDoc(w http.ResponseWriter, _ *http.Request, ps httprouter.Params) error {
	doc, err := s.getDocumentByID([]byte(ps.ByName("id")))
	if err != nil {
		return err
	}

	return jsonResponse(w, H{"document": doc})
}

func (s server) reindex() {
	s.Walk(func(key, val []byte) error {
		doc, err := UnmarshalJSON(val)
		if err != nil {
			log.Printf("Unable to parse bad document, %s: %s", key, err)
		}
		s.index(string(key), doc, true)
		return nil
	})
}

func (s *server) notifyFlush() {
	select {
	case s.flushNotify <- struct{}{}:
	default: // ignore
	}
}

func (s *server) flushing() {
	s.flushNotify = make(chan struct{})
	idleTimeout := time.NewTimer(10 * time.Second)
	defer idleTimeout.Stop()
	dirty := false

	for {
		idleTimeout.Reset(10 * time.Second)

		select {
		case <-s.flushNotify:
			dirty = true
		case <-idleTimeout.C:
			if dirty {
				s.Flush()
				dirty = false
			}
		}
	}
}

func logErr(err error) string {
	if err == nil {
		return "successfully"
	}
	return "failed: " + err.Error()
}
