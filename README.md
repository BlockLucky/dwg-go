# dwg-go

A Go library for reading DWG/DXF files.

This project provides:
- A **Go library** that exposes simple function APIs such as `ReadDWG`.
- A **local helper service (`dwg_service`)** used internally to process DWG files.

## Usage

```go
import "github.com/you/dwg-go"

doc, err := dwg.ReadDWG("a.dwg")
````