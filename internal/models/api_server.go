package models

type APIServer interface {
	Start()
	Shutdown() error
}

