module github.com/submariner-io/submariner-operator

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.3.2
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/prometheus-operator v0.38.1-0.20200424145508-7e176fda06cc
	github.com/go-errors/errors v1.2.0 // indirect
	github.com/go-logr/logr v0.4.0
	github.com/go-openapi/spec v0.20.3 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/gophercloud/utils v0.0.0-20210909165623-d7085207ff6d
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.14
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/openshift/api v0.0.0-20200324173355-9b3bdf846ea1
	github.com/openshift/cluster-dns-operator v0.0.0-20200529200012-f9e4dfc90c57
	github.com/operator-framework/operator-lib v0.4.0
	github.com/operator-framework/operator-sdk v0.19.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/spf13/cobra v1.3.0
	github.com/submariner-io/admiral v0.12.0-m3.0.20220211050139-69a40598bdd6
	github.com/submariner-io/cloud-prepare v0.12.0-m3.0.20220216201630-d2a9f9d88aae
	github.com/submariner-io/lighthouse v0.12.0-m3.0.20220215144325-3a00dd24acab
	github.com/submariner-io/shipyard v0.12.0-m3
	github.com/submariner-io/submariner v0.12.0-m3.0.20220118080926-1324572a65c0
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/uw-labs/lichen v0.1.5
	github.com/xlab/treeprint v1.1.0 // indirect
	go.starlark.net v0.0.0-20210506034541-84642328b1f0 // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	google.golang.org/api v0.70.0
	k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kustomize/cmd/config v0.9.11 // indirect
	sigs.k8s.io/kustomize/kustomize/v3 v3.10.0
	sigs.k8s.io/kustomize/kyaml v0.10.19 // indirect
	sigs.k8s.io/mcs-api v0.1.0
	sigs.k8s.io/structured-merge-diff/v4 v4.1.1 // indirect
	sigs.k8s.io/yaml v1.2.0
)

// When changing pins, check the dependabot configuration too
// in .github/dependabot.yml

// Pinned to kubernetes-1.19.10
replace (
	k8s.io/api => k8s.io/api v0.19.10
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.10
	k8s.io/client-go => k8s.io/client-go v0.19.10
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.10
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.7.0
)
