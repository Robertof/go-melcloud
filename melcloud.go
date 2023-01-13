package melcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
    loginUrl      = "https://app.melcloud.com/Mitsubishi.Wifi.Client/Login/ClientLogin"
    deviceInfoUrl = "https://app.melcloud.com/Mitsubishi.Wifi.Client/Device/Get"
    deviceListUrl = "https://app.melcloud.com/Mitsubishi.Wifi.Client/User/ListDevices"
)

type MelcloudRequestor struct {
    zerolog.Logger

    client *http.Client
    contextKey string
    reauthenticate func() (string, error)
}

func Authenticate(email, password string) (*MelcloudRequestor, error) {
    return AuthenticateWithLogger(email, password, log.With().Str("module", "go-melcloud").Logger())
}

func AuthenticateWithLogger(email, password string, log zerolog.Logger) (*MelcloudRequestor, error) {
    client := http.DefaultClient

    log.Info().Msg("Authenticating with MELCloud...")

    request := loginRequest{
        AppVersion:      "1.21.6.0",
        CaptchaResponse: nil,
        Email:           email,
        Password:        password,
        Language:        19,
        Persist:         true,
    }

    serializedRequest, _ := json.Marshal(request)
    rawResponse, err := client.Post(loginUrl, "application/json", bytes.NewBuffer(serializedRequest))

    if err != nil {
        return nil, fmt.Errorf("Unable to authenticate with MELCloud: %w", err)
    }

    defer rawResponse.Body.Close()

    log.Trace().
        Int("statusCode", rawResponse.StatusCode).
        Func(func(e *zerolog.Event) {
            res, _ := httputil.DumpResponse(rawResponse, true)
            e.Str("response", string(res))
        }).
        Msg("Received MELCloud login response")

    response := loginResponse{}
    if err = json.NewDecoder(rawResponse.Body).Decode(&response); err != nil {
        return nil, fmt.Errorf("Unable to decode MELCloud login response: %w", err)
    }

    if response.ErrorId != nil {
        return nil, fmt.Errorf("Unable to sign in to MELCloud, maybe your credentials are incorrect? (err: %v)", *response.ErrorId)
    }

    log.Info().Msg("Successfully authenticated with MELCloud")

    return &MelcloudRequestor{
        client:         client,
        contextKey:     response.LoginData.ContextKey,
        reauthenticate: func() (string, error) {
            result, err := AuthenticateWithLogger(email, password, log)
            if err != nil {
                return "", fmt.Errorf("Reauthentication failed: %w", err)
            }
            return result.contextKey, nil
        },
    }, nil
}

func (r *MelcloudRequestor) GetDeviceInformation(DeviceId, BuildingId string) (io.ReadCloser, error) {
    url, err := url.Parse(deviceInfoUrl)
    if err != nil {
        panic(err)
    }

    q := url.Query()
    q.Set("id", DeviceId)
    q.Set("buildingID", BuildingId)
    url.RawQuery = q.Encode()

    r.Logger.Debug().
        Str("url", url.String()).
        Str("deviceID", DeviceId).
        Str("buildingID", BuildingId).
        Msg("Requesting device info from MELCloud")

    return r.makeGet(url)
}

func (r *MelcloudRequestor) GetDeviceList() (io.ReadCloser, error) {
    url, err := url.Parse(deviceListUrl)
    if err != nil {
        panic(err)
    }

    r.Logger.Debug().
        Str("url", url.String()).
        Msg("Requesting device list from MELCloud")

    return r.makeGet(url)
}

func (r *MelcloudRequestor) makeGet(u *url.URL) (io.ReadCloser, error) {
    req, err := http.NewRequest("GET", u.String(), nil)
    if err != nil {
        return nil, fmt.Errorf("Unable to query MELCloud: %w", err)
    }

    res, err := r.makeRequest(req)
    if err != nil {
        return nil, fmt.Errorf("Unable to query MELCloud: %w", err)
    }

    r.Logger.Trace().
        Int("statusCode", res.StatusCode).
        Func(func(e *zerolog.Event) {
            res, _ := httputil.DumpResponse(res, true)
            e.Str("response", string(res))
        }).
        Msg("Received response from MELCloud")

    return res.Body, nil
}

func (r *MelcloudRequestor) makeRequest(req *http.Request) (*http.Response, error) {
    req.Header.Add("X-MitsContextKey", r.contextKey)
    req.Header.Add("Accept", "application/json")

    res, err := r.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("MELCloud request failed: %w", err)
    }

    if res.StatusCode == http.StatusUnauthorized {
        res.Body.Close()
        r.Logger.Warn().Msg("Performing MELCloud reauthentication")
        // Try to reauthenticate...
        contextKey, err := r.reauthenticate()
        if err != nil {
            return nil, err
        }
        r.contextKey = contextKey
        return r.makeRequest(req)
    }

    return res, nil
}
