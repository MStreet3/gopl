package main

import "fmt"

type ServerErrorType int

const (
	START_UP ServerErrorType = iota + 1
	SHUT_DOWN
)

type ReverbServerError struct {
	Err  error
	Type ServerErrorType
}

func NewReverbServerStartUpError(err error) *ReverbServerError {
	return &ReverbServerError{
		Err:  err,
		Type: START_UP,
	}
}

func NewReverbServerShutDownError(err error) *ReverbServerError {
	return &ReverbServerError{
		Err:  err,
		Type: SHUT_DOWN,
	}
}

func (e *ReverbServerError) Error() string {
	switch e.Type {
	case START_UP:
		return fmt.Errorf("failed to start server: %w", e.Err).Error()
	case SHUT_DOWN:
		return fmt.Errorf("failed to stop server: %w", e.Err).Error()
	default:
		return fmt.Sprintf("unknown error type")
	}
}
