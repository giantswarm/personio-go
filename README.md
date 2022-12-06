[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/personio-go/tree/main.svg?style=shield&circle-token=fa77270945b2f8a813060b9159a5c9a17c63bf05)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/personio-go/tree/main)

# personio-go

Simple net/http based Personio API client for go.

## Credentials File

The required credentials file for authenticating with Personio API v1 looks like follows:
```
{
        "clientId":"YOUR_CLIENT_ID",
        "clientSecret":"YOUR_CLIENT_SECRET"
}
```

## Usage Example

The following example exercises the `v1.GetEmployees()` and `v1.GetTimeOffs()` functions to dump all employees and time-offs.

To run this example, perform these steps:

1. Put the following code into a file named `main.go`  
    ```go
    package main
    
    import (
	    "context"
        "encoding/json"
        "log"
        "os"
    
        v1 "github.com/giantswarm/personio-go/v1"
    )
    
    // main dumps all data returned by the v1.GetEmployees() and v1.GetTimeOffs() functions to STDOUT
    func main() {
        credentials, err := os.ReadFile("personio-credentials.json")
        if err != nil {
            log.Fatal(err)
        }
    
        var personioCredentials v1.Credentials
        err = json.Unmarshal(credentials, &personioCredentials)
        if err != nil {
            log.Fatal(err)
        }
    
        personio, err := v1.NewClient(context.TODO(), v1.DefaultBaseUrl, personioCredentials)
    
        timeOffs, err := personio.GetTimeOffs(nil, nil)
        if err != nil {
            log.Fatal(err)
        }
    
        employees, err := personio.GetEmployees()
        if err != nil {
            log.Fatal(err)
        }
    
        type dump struct {
            Employees []*v1.Employee `json:"employees"`
            TimeOffs  []*v1.TimeOff  `json:"timeOffs"`
        }
        jsonDump, _ := json.MarshalIndent(dump{Employees: employees, TimeOffs: timeOffs}, "", "  ")
    
        os.Stdout.Write(jsonDump)
    }
    
    ```
3. Put `{"clientId": "CLIENT_ID", "clientSecret": "CLIENT_SECRET"}` into a file named `personio-credentials.json`
4. Run `go run main.go > output.json`
5. The file `output.json` should now contain the dumped data.

[generate]: https://github.com/giantswarm/personio-go/generate
