# dwg-go

## Licensing

- The Go library in the repository root is licensed under the **BSD 3-Clause License**.
- The `dwg_service` program under `dwg_service` is licensed under **GPLv3** and uses LibreDWG.

The Go library does not link against LibreDWG.  
All GPL-licensed code is isolated in the `dwg_service` program, which runs as a separate process.


## Usage

```go
import "github.com/you/dwg-go"

doc, err := dwg.ReadDWG("a.dwg")
```

## depend
### Windows
```shell

```