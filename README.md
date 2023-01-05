# go-melcloud

Barebones implementation of Melcloud's Read API in Golang. Supports authentication via email and password as well as automatic re-authentication when the (in-memory) persisted token expires.

Proper documentation and tests is TODO, please use this at your own risk...

## Quick start

```
go get github.com/robertof/go-melcloud
```

```go
import "github.com/robertof/go-melcloud"

const (
    deviceID = 123
    buildingID = 456
)

func main() {
    m, err := melcloud.Authenticate("email", "password")
    // or pass a zerolog.Logger instance:
    m, err := melcloud.AuthenticateWithLogger(
        "email",
        "password",
        log.With().Str("module", "go-melcloud").Logger(),
    )

    // Get information about a specific device
    info, err := m.GetDeviceInformation(deviceID, buildingID)
    defer info.Close()

    // info is a io.ReadCloser, use json.Unmarshal with your own model or just deserialize
    // to a dictionary
    var data map[string]interface{}

    err := json.NewDecoder(info).Decode(&data)
}
```

## See also

Check out this Prometheus exporter for Ecodan devices: https://github.com/Robertof/melcloud-prometheus-exporter (totally undocumented, but I didn't intend to make the repository public originally!).
