module kraftkit.sh

go 1.22
toolchain go1.22.2

require (
	api.zip v0.1.5
	github.com/AlecAivazis/survey/v2 v2.3.7
	github.com/LastPossum/kamino v0.0.1
	github.com/MakeNowJust/heredoc v1.0.0
	github.com/Masterminds/semver/v3 v3.2.1
	github.com/acorn-io/baaah v0.0.0-20230522221318-afcc93619e30
	github.com/anchore/stereoscope v0.0.2-0.20240229175558-fe426d1b1c84
	github.com/briandowns/spinner v1.23.0
	github.com/cavaliergopher/cpio v1.0.1
	github.com/charmbracelet/bubbles v0.18.0
	github.com/charmbracelet/bubbletea v0.25.0
	github.com/charmbracelet/lipgloss v0.10.0
	github.com/cli/cli/v2 v2.46.0
	github.com/cli/go-gh v1.2.1
	github.com/cli/safeexec v1.0.1
	github.com/compose-spec/compose-go v1.20.2
	github.com/compose-spec/compose-go/v2 v2.0.2
	github.com/containerd/containerd v1.7.15
	github.com/containerd/log v0.1.0
	github.com/containerd/nerdctl v1.7.5
	github.com/cyphar/filepath-securejoin v0.2.4
	github.com/dgraph-io/badger/v3 v3.2103.5
	github.com/docker/cli v26.0.2+incompatible
	github.com/docker/docker v26.0.2+incompatible
	github.com/dustin/go-humanize v1.0.1
	github.com/erikgeiser/promptkit v0.9.0
	github.com/erikh/ping v0.0.0-20141209185752-d731d249e12a
	github.com/firecracker-microvm/firecracker-go-sdk v1.0.0
	github.com/fsnotify/fsnotify v1.7.0
	github.com/go-git/go-git/v5 v5.12.0
	github.com/gobwas/glob v0.2.3
	github.com/google/go-containerregistry v0.19.1
	github.com/google/go-github/v32 v32.1.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.6.0
	github.com/henvic/httpretty v0.1.3
	github.com/kubescape/go-git-url v0.0.30
	github.com/mattn/go-colorable v0.1.13
	github.com/mattn/go-isatty v0.0.20
	github.com/mattn/go-shellwords v1.0.12
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/moby/buildkit v0.13.2
	github.com/muesli/reflow v0.3.0
	github.com/muesli/termenv v0.15.2
	github.com/onsi/ginkgo/v2 v2.17.2
	github.com/onsi/gomega v1.33.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0
	github.com/opencontainers/runc v1.1.12
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/opencontainers/selinux v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/rancher/wrangler v1.1.2
	github.com/shirou/gopsutil/v3 v3.24.3
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
	github.com/testcontainers/testcontainers-go v0.29.1
	github.com/vishvananda/netlink v1.2.1-beta.2.0.20231127184239-0ced8385386a
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/xlab/treeprint v1.2.0
	golang.org/x/oauth2 v0.19.0
	golang.org/x/sync v0.7.0
	golang.org/x/sys v0.19.0
	golang.org/x/term v0.19.0
	google.golang.org/grpc v1.59.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.29.3
	k8s.io/apimachinery v0.30.0
	k8s.io/apiserver v0.29.3
	oras.land/oras-go/v2 v2.5.0
	sdk.kraft.cloud v0.5.9-0.20240417173611-0430af5a2dac
	sigs.k8s.io/kustomize/kyaml v0.17.0
)

require (
	dario.cat/mergo v1.0.0 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20230811130428-ced1acdcaa24 // indirect
	github.com/AdamKorcz/go-118-fuzz-build v0.0.0-20230306123547-8075edf89bb0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Microsoft/hcsshim v0.11.4 // indirect
	github.com/ProtonMail/go-crypto v1.0.0 // indirect
	github.com/anchore/go-logger v0.0.0-20220728155337-03b66a5207d8 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/becheran/wildmatch-go v1.0.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.0.2 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chainguard-dev/git-urls v1.0.2 // indirect
	github.com/charmbracelet/harmonica v0.2.0 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/containerd/console v1.0.4 // indirect
	github.com/containerd/continuity v0.4.3 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/go-cni v1.1.9 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.15.1 // indirect
	github.com/containerd/ttrpc v1.2.3 // indirect
	github.com/containerd/typeurl/v2 v2.1.1 // indirect
	github.com/containernetworking/cni v1.1.2 // indirect
	github.com/containernetworking/plugins v1.4.0 // indirect
	github.com/coreos/go-iptables v0.7.0 // indirect
	github.com/cpuguy83/dockercfg v0.3.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.0 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.5.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/analysis v0.21.2 // indirect
	github.com/go-openapi/errors v0.20.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/loads v0.21.1 // indirect
	github.com/go-openapi/runtime v0.24.0 // indirect
	github.com/go-openapi/spec v0.20.4 // indirect
	github.com/go-openapi/strfmt v0.21.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/go-openapi/validate v0.22.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/goharbor/go-client v0.210.0 // indirect
	github.com/golang/glog v1.1.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/flatbuffers v23.1.21+incompatible // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20240424215950-a892ee059fd6 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/in-toto/in-toto-golang v0.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20230110061619-bbe2e5e100de // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/signal v0.7.0 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pierrec/lz4/v4 v4.1.17 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rootless-containers/rootlesskit v1.1.1 // indirect
	github.com/scylladb/go-set v1.0.3-0.20200225121959-cc7b2070d91e // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.4.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/skeema/knownhosts v1.2.2 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/sylabs/squashfs v0.6.1 // indirect
	github.com/therootcompany/xz v1.0.1 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/tonistiigi/fsutil v0.0.0-20240424095704-91a3fc46842c // indirect
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea // indirect
	github.com/tonistiigi/vt100 v0.0.0-20230623042737-f9a4f7ef6531 // indirect
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	github.com/wagoodman/go-partybus v0.0.0-20200526224238-eb215533f07d // indirect
	github.com/wagoodman/go-progress v0.0.0-20230925121702-07e42b3cdba0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.mongodb.org/mongo-driver v1.8.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.46.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.46.1 // indirect
	go.opentelemetry.io/otel v1.21.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.21.0 // indirect
	go.opentelemetry.io/otel/sdk v1.21.0 // indirect
	go.opentelemetry.io/otel/trace v1.21.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/exp v0.0.0-20240112132812-db7319d0e0e3 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
	google.golang.org/genproto v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
