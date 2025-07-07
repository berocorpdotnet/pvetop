# pvetop

A terminal-based monitoring tool for Proxmox VE, similar to `top` but for virtual machines and containers.

## Features

- Real-time monitoring of VMs and LXC containers
- Cluster detection with dedicated nodes view
- Display CPU, memory, disk I/O, and network I/O statistics
- Sort by VMID, name, CPU, or memory usage
- Filter to show only running VMs or all VMs
- Color-coded resource usage (green/yellow/red thresholds)
- Keyboard shortcuts for quick navigation
- Auto-refresh every 2 seconds

## Installation

### Prerequisites

- Go 1.21 or later
- Access to a Proxmox VE server

### Build from source

```bash
git clone https://github.com/berocorpdotnet/pvetop.git
cd pvetop
go mod download
go build -o pvetop
```

## Usage

Run pvetop:

```bash
./pvetop
```

First time run will prompt for ProxMox host details and username/password. The password isn't saved but used to generate an API token which is stored encrypted. 

To re-run the setup again if required:

```bash
./pvetop --setup
```

## Keyboard Shortcuts

- `q` or `Ctrl+C` - Quit
- `?` - Show help
- `a` - Toggle between showing all VMs or only active/running ones
- `n` - Switch between nodes view and guests view (cluster mode only)
- `v` - Sort by VMID
- `s` - Sort by name
- `c` - Sort by CPU usage
- `m` - Sort by memory usage
- `r` - Reverse sort order
