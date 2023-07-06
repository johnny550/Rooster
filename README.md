# Rooster
This tool's purpose is to automate the deployment of a daemonset and other related resources into a cluster.

# Available options
Option            | Type     |Required                          | Usage                                       | 
:-----------:     | :-------:|:--------------------------------:|:-------------------------------------------:|
cluster-id        | string   | true                             | current cluster's name                      |
increment         | int      | true if strategy=LINEAR          | Rollout increment over time (in percentage) |
decrement         | int      | true if scale-down               | How much to reduce resources by (in percentage) |
canary            | int      | true if strategy=CANARY          | canary batch size (in percentage)           |
canary-label      | string   | true                             | canary process control label                |
taget-label       | string   | true                             | existing label on nodes to target           |
project           | string   | true                             | Current project name                        |
version           | string   | false                            | Current project version                     |
update-if-exists  | bool     | false (unless action=update & manifestPath is given)     | Overwrite existing resources                |
strategy          | string   | false                            | desired rollout strategy. Canary, or linear |
manifest-path     | string   | false                            | YAML manifests path                         |
test-suite        | string   | false                            | name of the test suite                      |
test-binary       | string   | false                            | test suite, or function name                |
namespace         | string   | false                            | targeted namespace                          |
dry-run           | bool     | false                            | dry-run                                     |

test-package and test-binary options must either both be specified, or not at all. Specifying only one will result in an error being triggered.

# How to start
## View the available options
```
go run rooster/cmd/manager -h
```
## Execution command
### Rollout
In charge of:
- Deploying resources onto a cluster with calculated scope, if the path to the manifests is indicated
- Extending the scope of currently deployed resources
#### Strategy: Linear (Defaul rollout strategy)
```
go run rooster/cmd/manager \
--increment <INTEGER> \
--target-label <EXISTING-LABEL-TO-TARGET> \
--canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> \
--manifest-path /path/to/files \
--cluster-id <CLUSTER_NAME> \
--project <PROJECT-NAME> \
--version <VERSION-NAME> \
--action rollout
```
#### Strategy: Canary
```
go run rooster/cmd/manager \
--strategy canary  \
--canary <INTEGER> \
--target-label <EXISTING-LABEL-TO-TARGET> \
--canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> \
--manifest-path /path/to/files \
--cluster-id <CLUSTER_NAME> \
--project <PROJECT-NAME> \
--version <VERSION-NAME> \
--action rollout
```
### Rollback
#### To a specific version
The version needs to be known to Rooster, meaning a previous backup was made \
With this command, no need to specify a manifest-path. It will be determined by Rooster itself, using the given version.
```
go run rooster/cmd/manager \
--target-label <EXISTING-LABEL-TO-TARGET> \
--canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> \
--cluster-id <CLUSTER_NAME> \
--project <PROJECT-NAME> \
--version <PREVIOUS-VERSION> \
--action rollback
```
### Clean resources
Remove all resources from the cluster, clean labels as well. \
Indicating a path to the manifests means resources will also be removed from the cluster.
```
go run rooster/cmd/manager \
--target-label <EXISTING-LABEL-TO-TARGET> \
--canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> \
--manifest-path /path/to/files \
--cluster-id <CLUSTER_NAME> \
--project <PROJECT-NAME> \
--action rollback
```
## Update
Rollout a version following another. \
### Update and patch resources
Indicating a manifests path means resources configuration will be patched.
```
go run rooster/cmd/manager \
--increment <HOW-MANY-PODS-OF-A-DAEMONSET-TO-UPDATE-AT-ONCE> \
--target-label <EXISTING-LABEL-TO-TARGET> \
--canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> \
--manifest-path /path/to/files \
--cluster-id <CLUSTER_NAME> \
--project <PROJECT-NAME> \
--version <NEXT-VERSION> \
--update-if-exists=true \
--action update
```
### Update nodes & CM but not other resources
```
go run rooster/cmd/manager \
--increment <HOW-MANY-PODS-OF-A-DAEMONSET-TO-UPDATE-AT-ONCE> \
--target-label <EXISTING-LABEL-TO-TARGET> \
--canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> \
--cluster-id <CLUSTER_NAME> \
--project <PROJECT-NAME> \
--version <NEXT-VERSION> \
--action update
```

## Scale down
Reduce the scope of a rollout. Lessen the number of target nodes.
```
go run rooster/cmd/manager \
--decrement <INTEGER> \
--target-label <EXISTING-LABEL-TO-TARGET> \
--canary-label <LABEL-TO-CONTROL-THE-CANARY-PROCESS> \
--cluster-id <CLUSTER_NAME> \ 
--project <PROJECT-NAME> \ 
--action scale-down
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
4. Use the tesdt-binary and test-suite options in the rollout command

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
go clean -testcache
go test -v rooster/pkg/tests rooster/pkg/worker
go test -v rooster/pkg/tests rooster/pkg/worker -coverpkg=./... -coverprofile=cover.out && go tool cover -html=cover.out -o cover.html && open cover.html
```

# Versioning config maps
These are created for each project. They contain all the data necessary for verifying the state of a project and its versions. \
Here is an example of one. \
```
project: myProject
info:
  - version: version1
    current: "false"
    nodes:
      - node4
      - node5
      - node6
  - version: version2
    current: "true"
    nodes:
      - node1
      - node2
```

# Future improvements
* Create a Makefile, chain it to a script to easily execute tests, build the environment, and other useful operations. CAP-4579
* Ability to scale down a version that is not current. At this point, scaling down a version that isn't current is forbidden. Find a way around that. CAP-4444
* Stop the rollout action from scaling up resources. Isolate such ability in a new action: scale-up CAP-4580
* Improve how manifest files are checked. When a file is empty, Rooster ignores it, but when it doesn't contain YAML or misses critical fields, v3.Yaml hangs at yaml.NewDecoder(*io.Reader).Decode(&struct) CAP-4295
* Extend the rollout methods to be automatically done by Rooster after a user-specified time (linear-10-Every-Minute, Canary10Percent10Minutes, etc...) CAP-4294
* Accept other test binaries. Language-agnosticism is the goal. CAP-4293
* Consider keeping backup files in a more reliable & available location: bitbucket, s3, etc... CAP-4292
* Improve the support for multiple daemonsets. So far multiple daemonsets can be released at once, as long as they define the same labels in their affinity context. CAP-4291
* Idempotency. Rooster will be run as a controller from inside the cluster, as operator/controller, in the future. (same idea of deployment-controller over replicaset. Rooster can work as a controller for daemonset) CAP-4296
* Can I get rid of the kubectlEmulator? CAP-4443
* Improve the code quality of the function in charge of checking the readiness of resources (areResourcesReady) CAP-4297
* ~~Validate the canary-label by making sure it does not exist on any node in the cluster before going any further (Early failure policy).~~
* ~~Improve the backup folder logic, and set backup files to be kept in different folders, based off the date, cluster name, etc...~~
* ~~Make the canary fashion optional. Allow-Rooster to deploy resources in different ways (all-at-once, canary-10Minutes, linear-10-Every-Minute, etc...)~~
* ~~Add support for updates (updatePolicy: onDelete, and deletes pods?). Not just new releases but also existing resources being updated and released~~
* ~~Add the ability to trigger a rollback on demand~~

# License
Licensed under the Apache License, Version 2.0