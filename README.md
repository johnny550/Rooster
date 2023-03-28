# Rooster
This tool's purpose is to automate the deployment of a Kubernetes daemonset and other related resources into a cluster.

# Available options
Option        | Type     |Required  | Usage                         | 
:-----------: | :-------:|:--------:|:---------------------------------:|
namespace     | string   | false    | targeted namespace (optional)     |
canary        | int      | true     | canary batch size (in percentage) |
canary-label  | string   | true     | canary process control label      |
taget-label   | string   | true     | existing label on nodes to target |
manifest-path | string   | true     | YAML manifests path               |
test-package  | string   | true     | name of the test package          |
test-binary   | string   | true     | test suite, or function name      |
dry-run       | string   | false    | dry-run                           |

# How to start
## Execution command
```
go run cmd/manager/main.go --canary <CANARY-BATCH-SIZE> --target-label <EXISTING-LABEL-TO-TARGET> --canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> --manifest-path /path/to/files --test-package <TEST_SUITE_OR_FUNCTION_NAME> --test-binary <BINARY_NAME>
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
go run cmd/manager/main.go --canary 50 --target-label aaa=bbb --canary-label xxx=yyy--manifest-path /~/Documents/projects/myproject/ --test-package XxxxYyy
```

# Unit tests
To run the test, use the following command
```
go test -v rooster/pkg/tests 
```

# Future improvements
* Make the canary fashion optional. Allow Caas-Rooster to deploy resources in different ways (all-at-once, canary-10Minutes, linear-10-Every-Minute, etc...)
* Accept other test binaries. Language-agnosticism is the goal.
* Add support for updates (updatePolicy: onDelete, and deletes pods?). Not just new releases but also existing resources being updated and released in a canary mode
* Consider keeping backup files in a more reliable & available location: bitbucket, s3, etc...
* Improve the support for multiple daemonsets. So far multiple daemonsets can be released at once, as long as they define the same labels in their affinity context.
* ~~Validate the canary-label by making sure it does not exist on any node in the cluster before going any further (Early failure policy).~~
* Idempotency, deterministic behavior. Rooster will be run as a controller from inside the cluster, as operator/controller, in the future. (same idea of deployment-controller over replicaset. streamlinear can work as a controller for daemonset)
* Improve the backup folder logic, and set backup files to be kept in different folders, based off the date, cluster name, etc...
* Add the ability to trigger a rollback on demand