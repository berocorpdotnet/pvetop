package models

import "time"

type Node struct {
	Node   string `json:"node"`
	Status string `json:"status"`
	CPU    float64 `json:"cpu"`
	MaxCPU int     `json:"maxcpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxmem"`
	Disk   int64   `json:"disk"`
	MaxDisk int64  `json:"maxdisk"`
	Uptime int64   `json:"uptime"`
}

type Guest struct {
	VMID     int     `json:"vmid"`
	Name     string  `json:"name"`
	Type     string  `json:"type"` 
	Status   string  `json:"status"`
	Node     string  `json:"node"`
	CPU      float64 `json:"cpu"`
	CPUs     int     `json:"cpus"`
	Mem      int64   `json:"mem"`
	MaxMem   int64   `json:"maxmem"`
	Disk     int64   `json:"disk"`
	MaxDisk  int64   `json:"maxdisk"`
	NetIn    int64   `json:"netin"`
	NetOut   int64   `json:"netout"`
	DiskRead int64   `json:"diskread"`
	DiskWrite int64  `json:"diskwrite"`
	Uptime   int64   `json:"uptime"`
	PID      int     `json:"pid,omitempty"`
}

type GuestStatus struct {
	VMID      int     `json:"vmid"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	CPUs      int     `json:"cpus"`
	Mem       int64   `json:"mem"`
	MaxMem    int64   `json:"maxmem"`
	Disk      int64   `json:"disk"`
	MaxDisk   int64   `json:"maxdisk"`
	NetIn     int64   `json:"netin"`
	NetOut    int64   `json:"netout"`
	DiskRead  int64   `json:"diskread"`
	DiskWrite int64   `json:"diskwrite"`
	Uptime    int64   `json:"uptime"`
	PID       int     `json:"pid,omitempty"`
	UpdatedAt time.Time
}
