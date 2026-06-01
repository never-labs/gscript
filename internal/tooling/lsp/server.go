package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

const (
	jsonrpcVersion = "2.0"

	errCodeParseError     = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
)

// Server is a minimal Leia Language Server Protocol endpoint.
type Server struct {
	mu        sync.Mutex
	docs      map[string]string
	shutdown  bool
	writer    *bufio.Writer
	writeLock sync.Mutex
}

// NewServer creates a server with an empty in-memory document set.
func NewServer() *Server {
	return &Server{docs: map[string]string{}}
}

// Run serves LSP JSON-RPC messages from r and writes responses to w.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	s.writer = bufio.NewWriter(w)
	br := bufio.NewReader(r)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		payload, err := readMessage(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := s.handle(payload); err != nil {
			return err
		}
	}
}

type requestMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type responseMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type notificationMessage struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func (s *Server) handle(payload []byte) error {
	var req requestMessage
	if err := json.Unmarshal(payload, &req); err != nil {
		return s.writeError(rawID("null"), errCodeParseError, err.Error())
	}
	if req.JSONRPC != "" && req.JSONRPC != jsonrpcVersion {
		return s.respondMaybe(req.ID, nil, &responseError{Code: errCodeInvalidRequest, Message: "unsupported jsonrpc version"})
	}
	if req.Method == "" {
		return s.respondMaybe(req.ID, nil, &responseError{Code: errCodeInvalidRequest, Message: "missing method"})
	}

	switch req.Method {
	case "initialize":
		return s.respondMaybe(req.ID, initializeResult(), nil)
	case "initialized":
		return nil
	case "shutdown":
		s.mu.Lock()
		s.shutdown = true
		s.mu.Unlock()
		return s.respondMaybe(req.ID, nil, nil)
	case "exit":
		return io.EOF
	case "textDocument/didOpen":
		return s.didOpen(req.Params)
	case "textDocument/didChange":
		return s.didChange(req.Params)
	case "textDocument/didClose":
		return s.didClose(req.Params)
	default:
		return s.respondMaybe(req.ID, nil, &responseError{Code: errCodeMethodNotFound, Message: "method not found: " + req.Method})
	}
}

func initializeResult() map[string]any {
	// TODO(lsp): add format, completion, hover, and navigation handlers as
	// shared tooling APIs become available outside cmd/leia internals.
	return map[string]any{
		"capabilities": map[string]any{
			"textDocumentSync": map[string]any{
				"openClose": true,
				"change":    1,
			},
		},
		"serverInfo": map[string]any{
			"name":    "leia-lsp",
			"version": "0.1.0",
		},
	}
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId,omitempty"`
	Version    int    `json:"version,omitempty"`
	Text       string `json:"text"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version,omitempty"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type textDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

func (s *Server) didOpen(params json.RawMessage) error {
	var p didOpenParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if p.TextDocument.URI == "" {
		return nil
	}
	diagnostics := syntaxDiagnostics(p.TextDocument.Text)
	s.mu.Lock()
	s.docs[p.TextDocument.URI] = p.TextDocument.Text
	s.mu.Unlock()
	return s.publishDiagnostics(p.TextDocument.URI, diagnostics)
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []textDocumentContentChangeEvent `json:"contentChanges"`
}

func (s *Server) didChange(params json.RawMessage) error {
	var p didChangeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if p.TextDocument.URI == "" || len(p.ContentChanges) == 0 {
		return nil
	}
	text := p.ContentChanges[len(p.ContentChanges)-1].Text
	diagnostics := syntaxDiagnostics(text)
	s.mu.Lock()
	s.docs[p.TextDocument.URI] = text
	s.mu.Unlock()
	return s.publishDiagnostics(p.TextDocument.URI, diagnostics)
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

func (s *Server) didClose(params json.RawMessage) error {
	var p didCloseParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if p.TextDocument.URI == "" {
		return nil
	}
	s.mu.Lock()
	delete(s.docs, p.TextDocument.URI)
	s.mu.Unlock()
	return s.publishDiagnostics(p.TextDocument.URI, nil)
}

func (s *Server) publishDiagnostics(uri string, diagnostics []diagnostic) error {
	if diagnostics == nil {
		diagnostics = []diagnostic{}
	}
	return s.write(notificationMessage{
		JSONRPC: jsonrpcVersion,
		Method:  "textDocument/publishDiagnostics",
		Params: map[string]any{
			"uri":         uri,
			"diagnostics": diagnostics,
		},
	})
}

func (s *Server) respondMaybe(id *json.RawMessage, result any, respErr *responseError) error {
	if id == nil {
		return nil
	}
	if respErr != nil {
		return s.write(responseMessage{
			JSONRPC: jsonrpcVersion,
			ID:      *id,
			Error:   respErr,
		})
	}
	// JSON-RPC success responses must include result; shutdown returns result:null.
	return s.write(map[string]any{
		"jsonrpc": jsonrpcVersion,
		"id":      *id,
		"result":  result,
	})
}

func (s *Server) writeError(id json.RawMessage, code int, message string) error {
	return s.write(responseMessage{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error:   &responseError{Code: code, Message: message},
	})
}

func (s *Server) write(msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.writeLock.Lock()
	defer s.writeLock.Unlock()
	if s.writer == nil {
		return errors.New("lsp: server writer is not initialized")
	}
	if _, err := fmt.Fprintf(s.writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	if _, err := s.writer.Write(payload); err != nil {
		return err
	}
	return s.writer.Flush()
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("lsp: malformed header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("lsp: invalid Content-Length %q", strings.TrimSpace(value))
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, errors.New("lsp: missing Content-Length")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func rawID(s string) json.RawMessage {
	return json.RawMessage([]byte(s))
}

func encodeMessage(v any) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(payload))
	buf.Write(payload)
	return buf.Bytes(), nil
}
