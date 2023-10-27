# kubevirt-ip-helper-migration-tool

This tool helps transfering the current allocated DHCP leases into vmnetcfg objects.
It collects the IP addresses from the KubeVirt VirtualMachineInstance (VMI) objects and generates the vmnetcfg objects.

> **_NOTE:_** when migrating from an existing DHCP service to the kubevirt-ip-helper make sure that the server IP address is configured
on a different address then the old DHCP service to prevent clients rejecting the new DHCP offers.

## Building the tool

Execute the go build command to build the tool:
```SH
go build -o kubevirt-ip-helper-migration-tool .
```

## Usage

Make sure the kubeconfig of the KubeVirt cluster is used or point the KUBECONFIG environment variable to it, for example:
```SH
export KUBECONFIG=<PATH_TO_KUBECONFIG_FILE>
```

When you run the migration tool you need to specify the subnet cidr of the  DHCP subnet, for example:
```SH
./kubevirt-ip-helper-migration-tool 192.168.10.0/24
```

## DHCP migration steps

1. Import the kubevirt-ip-helper CRD, for example:
```SH
kubectl create -f https://raw.githubusercontent.com/joeyloman/kubevirt-ip-helper/main/deployments/crds.yaml
```

2. Make sure that all the Virtual Machines are running and have an active IP address lease of the "old" DHCP service.

3. Shutdown the "old" DHCP service.

4. Run the kubevirt-ip-helper-migration-tool like described above in the Usage.

5. Check if all the leased IP addresses are added to the vmnetcfg objects:
 ```SH
 kubectl get vmnetcfg -A -o json | jq '.items[].spec.networkconfig[].ipaddress' -r | sort -V
 ```

6. Deploy the kubevirt-ip-helper and make sure it listens on a **different address** then the "old" DHCP service.

7. Create the matching IPPool object with the subnet DHCP configuration like described in the README of the kubevirt-ip-helper repository.

# License

Copyright (c) 2023 Joey Loman <joey@binbash.org>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.