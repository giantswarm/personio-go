[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/personio-go/tree/main.svg?style=shield&circle-token=fa77270945b2f8a813060b9159a5c9a17c63bf05)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/personio-go/tree/main)

# personio-go

Simple net/http based Personio API client for go.


## Credentials File

### API v1

Format: ```{"clientId":"YOUR_CLIENT_ID", "clientSecret":"YOUR_CLIENT_SECRET"}```

## USAGE

Use in your go program like this:
```go
package dosomething

import (
	"os"
	"encoding/json"
	
	"github.com/giantswarm/personio-go/v1"
)

func DoSomethingWithPersonio() error {
    credentials, err := os.ReadFile("personio-credentials.json")
    if err != nil {
        return err
    }
    
    var personioCredentials v1.Credentials
    err = json.Unmarshal(credentials, &personioCredentials)
    if err != nil {
		return err
    }
    
    personio, err := v1.NewClient(nil, personioCredentials)
    // ...
    
    return nil
}
```


[generate]: https://github.com/giantswarm/personio-go/generate
