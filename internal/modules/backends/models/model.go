package models

type Backend struct {
	Id     uint64
	URL    string
	Health string
}

type BackendStatus struct {
	Id        uint64
	IsHealthy bool
}
