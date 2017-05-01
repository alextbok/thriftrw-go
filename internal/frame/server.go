// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package frame

import (
	"fmt"
	"io"

	"go.uber.org/atomic"
)

// Handler handles incoming framed requests.
type Handler interface {
	// Receives the given framed request and responds to it.
	Handle([]byte) ([]byte, error)
}

// Server provides bidirectional incoming framed communication.
//
// It allows receiving framed requests and responding to them.
type Server struct {
	r *Reader
	w *Writer

	running *atomic.Bool

	shouldCloseReader atomic.Bool
	shouldCloseWriter atomic.Bool
}

// NewServer builds a new server which reads requests from the given Reader
// and writes responses to the given Writer.
func NewServer(r io.Reader, w io.Writer) *Server {
	return &Server{
		r:       NewReader(r),
		w:       NewWriter(w),
		running: atomic.NewBool(false),
	}
}

// Serve serves the given Handler with the Server.
//
// Only one request is served at a time. The server stops handling requests if
// there is an IO error or an unhandled error is received from the Handler.
//
// This blocks until the server is stopped using Stop.
func (s *Server) Serve(h Handler) (err error) {
	if s.running.Swap(true) {
		return fmt.Errorf("server is already running")
	}

	defer s.closeReaderWriter()

	for s.running.Load() {
		req, err := s.r.Read()
		if err != nil {
			// If the error occurred because the server was stopped, ignore it.
			if !s.running.Load() {
				break
			}

			return err
		}

		res, err := h.Handle(req)
		if err != nil {
			return err
		}

		if err := s.w.Write(res); err != nil {
			return err
		}
	}

	return nil
}

// Stop tells the Server that it's okay to stop Serve.
//
// This is a no-op if the server wasn't already running.
func (s *Server) Stop() error {
	if s.running.Swap(false) {
		s.closeReader()
	}
	return nil
}

func (s *Server) closeReader() {
	s.shouldCloseReader.Store(true)
	if s.shouldCloseWriter.Load() {
		s.closeReaderWriter()
	}
}

func (s *Server) closeReaderWriter() {
	s.r.Close()
	s.w.Close()
}
