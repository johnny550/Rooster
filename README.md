# Rooster
This tool's purpose is to automate the deployment of a daemonset and other related resources into a cluster.

# Available options
Option            | Type     |Required                  | Usage                                       | 
:-----------:     | :-------:|:------------------------:|:-------------------------------------------:|
cluster-name      | string   | true                     | current cluster's name                      |
strategy          | string   | true                     | desired rollout strategy. Canary, or linear |
update-if-exists  | bool     | false                    | Update existing resources                   |
increment         | int      | true if strategy=LINEAR  | Rollout increment over time (in percentage) |
canary            | int      | true if strategy=CANARY  | canary batch size (in percentage)           |
canary-label      | string   | true                     | canary process control label                |
taget-label       | string   | true                     | existing label on nodes to target           |
manifest-path     | string   | true                     | YAML manifests path                         |
test-package      | string   | false                    | name of the test package                    |
test-binary       | string   | false                    | test suite, or function name                |
namespace         | string   | false                    | targeted namespace                          |
dry-run           | bool     | false                    | dry-run                                     |

test-package and test-binary options must either both be specified, or not at all. Specifying only one will result in an error being triggered.

# How to start
## View the available options
```
go run rooster/cmd/manager -h
```
## Execution command
```
go run rooster/cmd/manager ---strategy canary --canary <CANARY-BATCH-SIZE> --target-label <EXISTING-LABEL-TO-TARGET> --canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> --manifest-path /path/to/files --test-package <TEST_SUITE_OR_FUNCTION_NAME> --test-binary <BINARY_NAME> --cluster-name <CLUSTER_NAME>

go run rooster/cmd/manager ---strategy linear --target-label <EXISTING-LABEL-TO-TARGET> --increment <INCREMENT_INT> --manifest-path /path/to/files --test-package <TEST_SUITE_OR_FUNCTION_NAME> --test-binary <BINARY_NAME> --cluster-name <CLUSTER_NAME>
```

## How to plug your tests in?
A part from some basic checks regarding the status of the resources it deploys for you, Streamline does not define validating test for you resources. That responsibility is yours.\
Nonetheless, Rooster would execute a properly compiled Golang test binary and return the output in the command line.\
To use your specific test, please do the following:\
1. build your test file
```
go test -v -c -o <CUSTOM-NAME>
```
2. put the binary in ___pkg/test-cases___ \
In your test file, you may use the k8s client-go to build resources, but if you wish to build resources using simple yaml files,
indicate the path to them using an environment variable. \
3. Set the environment variable specifying the path to your manifest files \
Before executing Rooster, do not forget to export the environment variable you set. The environment variable you set, should be the same in your test binary and for Rooster.

#### ___Example___
#### In test file 
podManifestPath := os.Getenv("M_PATH")
cmd, err := kubectl(sanboxNamespace, "apply", podManifestPath)
#### When running your tests through Rooster
```
Create your the necessary manifest test files at pkg/testdata/dns/dnsutil.yaml
export M_PATH="pkg/testdata/dns/dnsutil.yaml"
go run cmd/manager/main.go --strategy canary --canary 50 --target-label cluster.aps.cpd.rakuten.com/noderole=worker --canary-label xxx=yyy--manifest-path /~/Documents/projects/myproject/ --test-package XxxxYyy
```

# Unit tests
To run the test, use the following command
```
go test -v rooster/pkg/tests
go test -v rooster/pkg/tests rooster/pkg/worker -coverpkg=./... -coverprofile=cover.out && go tool cover -html=cover.out -o cover.html && open cover.html
```

# Future improvements
* Create a Makefile, chain it to a script to easily execute tests, build the environment, and other useful operations.
* Improve how manifest files are checked. When a file is empty, Rooster ignores it, but when it doesn't contain YAML or misses critical fields, v3.Yaml hangs at yaml.NewDecoder(*io.Reader).Decode(&struct)
* Extend the rollout methods to be automatically done by Rooster after a user-specified time (linear-10-Every-Minute, Canary10Percent10Minutes, etc...)
* Accept other test binaries. Language-agnosticism is the goal.
* Add support for updates (updatePolicy: onDelete, and deletes pods?). Not just new releases but also existing resources being updated and released in a canary mode
* Consider keeping backup files in a more reliable & available location: bitbucket, s3, etc...
* Improve the support for multiple daemonsets. So far multiple daemonsets can be released at once, as long as they define the same labels in their affinity context.
* Idempotency. Rooster will be run as a controller from inside the cluster, as operator/controller, in the future. (same idea of deployment-controller over replicaset. streamlinear can work as a controller for daemonset)
* Add the ability to trigger a rollback on demand
* Improve the code quality of the function in charge of checking the readiness of resources (areResourcesReady)
* ~~Validate the canary-label by making sure it does not exist on any node in the cluster before going any further (Early failure policy).~~
* ~~Improve the backup folder logic, and set backup files to be kept in different folders, based off the date, cluster name, etc...~~
* ~~Make the canary fashion optional. Allow-Rooster to deploy resources in different ways (all-at-once, canary-10Minutes, linear-10-Every-Minute, etc...)~~

# License
Licensed under the Apache License, Version 2.0