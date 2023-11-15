package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/joeyloman/kubevirt-ip-helper/pkg/util"

	kihv1 "github.com/joeyloman/kubevirt-ip-helper/pkg/apis/kubevirtiphelper.k8s.binbash.org/v1"
	kihclientset "github.com/joeyloman/kubevirt-ip-helper/pkg/generated/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtV1 "kubevirt.io/api/core/v1"
)

func createVmnetcfgObject(kih_clientset *kihclientset.Clientset, vmnetcfg *kihv1.VirtualMachineNetworkConfig) (err error) {
	vmNetCfgObj, err := kih_clientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(vmnetcfg.Namespace).Create(context.TODO(), vmnetcfg, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("cannot create VirtualMachineNetworkConfig object for vm [%s/%s]: %s",
			vmnetcfg.ObjectMeta.Namespace, vmnetcfg.ObjectMeta.Name, err.Error())
	}

	log.Printf("successfully create vmnetcfg object: %s/%s", vmNetCfgObj.Namespace, vmNetCfgObj.Name)

	return
}

func gatherKubevirtNetworkConfiguration(k8s_clientset *kubernetes.Clientset, kih_clientset *kihclientset.Clientset, subnet string) (err error) {
	ipnet, err := netip.ParsePrefix(subnet)
	if err != nil {
		return err
	}

	vmis, err := k8s_clientset.RESTClient().Get().AbsPath("/apis/kubevirt.io/v1").Namespace(corev1.NamespaceAll).Resource("virtualmachineinstances").DoRaw(context.TODO())
	if err != nil {
		return fmt.Errorf("error: while fetching vmi objects: %s", err.Error())
	}

	v := kubevirtV1.VirtualMachineInstanceList{}
	if err = json.Unmarshal(vmis, &v); err != nil {
		return fmt.Errorf("error: while unmarshalling json: %s", err.Error())
	}

	for _, vmi := range v.Items {
		vmnetcfg := kihv1.VirtualMachineNetworkConfig{}
		vmnetcfg.ObjectMeta.Name = vmi.ObjectMeta.Name
		vmnetcfg.ObjectMeta.Namespace = vmi.ObjectMeta.Namespace
		finalizers := []string{}
		finalizers = append(finalizers, "kubevirtiphelper")
		vmnetcfg.ObjectMeta.Finalizers = finalizers
		vmnetcfg.Spec.VMName = vmi.ObjectMeta.Name

		netCfgs := []kihv1.NetworkConfig{}

		for _, int := range vmi.Status.Interfaces {
			intIP, err := netip.ParseAddr(int.IP)

			if err == nil {
				if ipnet.Contains(intIP) {
					log.Printf("processing vmi -> name=%s,mac,%s,interface=%s,ip=%s", vmi.Name, int.MAC, int.InterfaceName, int.IP)

					if int.MAC == "" {
						log.Printf("error: no mac address found, skipping interface!")
					} else {
						for _, nic := range vmi.Spec.Networks {
							//log.Printf("processing network[%d] -> name=%s", k, nic.Name)
							if int.Name == nic.Name {
								if nic.Multus == nil {
									log.Printf("error: unsupported network type found, skipping interface!")
								} else if nic.Multus.NetworkName == "" {
									log.Printf("error: no networkname found, skipping interface!")
								} else {
									//log.Printf("network name found: %s, adding interface", nic.Multus.NetworkName)
									netCfg := kihv1.NetworkConfig{}
									netCfg.IPAddress = int.IP
									netCfg.MACAddress = int.MAC
									netCfg.NetworkName = nic.Multus.NetworkName
									netCfgs = append(netCfgs, netCfg)
								}
							}
						}
					}
				}
			}

		}
		vmnetcfg.Spec.NetworkConfig = netCfgs

		if err := createVmnetcfgObject(kih_clientset, &vmnetcfg); err != nil {
			log.Printf("error: while creating vmnetcfg object: %s", err.Error())
		}
	}

	return
}

func main() {
	if len(os.Args) < 2 {
		log.Printf("usage: %s <subnet cidr>", os.Args[0])
		os.Exit(1)
	}
	subnet := os.Args[1]

	var kubeconfig_file string
	kubeconfig_file = os.Getenv("KUBECONFIG")
	if kubeconfig_file == "" {
		kubeconfig_file = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	var config *rest.Config
	if util.FileExists(kubeconfig_file) {
		// uses kubeconfig
		kubeconfig := flag.String("kubeconfig", kubeconfig_file, "(optional) absolute path to the kubeconfig file")
		flag.Parse()
		config_kube, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
		config = config_kube
	} else {
		// creates the in-cluster config
		config_rest, err := rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		config = config_rest
	}

	k8s_clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	kih_clientset, err := kihclientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	if err := gatherKubevirtNetworkConfiguration(k8s_clientset, kih_clientset, subnet); err != nil {
		log.Printf("error: %s", err.Error())
	}
}
