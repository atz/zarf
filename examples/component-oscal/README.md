# Include OSCAL files in a Zarf package

This allows Zarf package developers to know what compliance controls their Zarf package satisfies.

## Walkthrough

1. Get the code

    If you already have the Zarf repository cloned locally and are in the root of the repository, switch to the `zarf-oscal` branch to follow this walkthrough:

    ```bash
    git switch zarf-oscal
    ```

    If you don't have the Zarf repository cloned locally, clone the repository with the `zarf-oscal` branch checked out:

    ```bash
    git clone --branch zarf-oscal https://github.com/defenseunicorns/zarf.git
    ```

1. Examine the artifacts

    Change into the `examples/component-oscal` directory:

    ```bash
    cd examples/component-oscal
    ```

    In this directory, you will see a `zarf.yaml` file:

    ```yaml
    kind: ZarfPackageConfig
    metadata:
      name: oscal-example
      description: Demo Zarf package composability with OSCAL data
      architecture: amd64
      version: v0.0.1

    components:
      - name: oscal-data
        required: true
        description: OSCAL control inheritance data for Kyverno
        oscal:
          - source: https://repo1.dso.mil/big-bang/product/packages/kyverno/-/raw/main/oscal-component.yaml
            destination: ./kyverno/oscal-component.yaml

      - name: kyverno
        required: true
        description: Deploy Kyverno as a Helm chart with Zarf
        charts:
          - name: kyverno
            namespace: kyverno
            url: oci://registry1.dso.mil/bigbang/kyverno
            version: 2.6.5-bb.3
            valuesFiles:
              - values.yaml
        images:
          - registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.5
          - registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.5
    ```

    Note the `components.oscal` field. This field is used to specify the OSCAL files that Zarf should include in this package.

    The `components.oscal.source` field is used to tell Zarf where to find and fetch an OSCAL file from. This can be either a path to a file on the local filesystem, or a remote URL that points to a raw OSCAL file for Zarf to fetch.

    The `components.oscal.destination` field is used to tell Zarf what path or directory to put the OSCAL files in the Zarf package bundle.

1. Create the Zarf package with the code changes on this branch by running `go run`:

    ```bash
    go run ../../main.go package create --confirm
    ```

1. Examine the output:

    ```bash
    COMPONENT       CONTROL
    Kyverno         cm-4
    Kyverno         cm-4.1
    Kyverno         cm-6
    Kyverno         cm-7
    Kyverno         cm-7.5
    Kyverno         cm-8.3
    Kyverno         cm-8.3
    Kyverno         sr-11
    ```

    The output shows which controls are satisfied by the application based on the OSCAL files included in the Zarf package.

1. Extract the Zarf package:

    ```bash
    zarf tools archiver decompress zarf-package-oscal-example-amd64.tar.zst kyverno
    ```

1. Extract the `oscal-data.tar` tarball:

    ```bash
    tar -xf kyverno/components/oscal-data.tar
    ```

1. Examine the Kyverno OSCAL file:

    ```bash
    cat oscal-data/oscal/kyverno/oscal-component.yaml 
    ```

    ```yaml
    component-definition:
    uuid: 839794c7-32c4-4329-b05c-6acd53de20ee
    metadata: 
      title: Kyverno Component
      last-modified: '2022-04-13T12:00:00Z'
      version: "20220413"
      oscal-version: 1.0.0
      parties:
        # Should be consistent across all of the packages, but where is ground truth?
      - uuid: 72134592-08C2-4A77-ABAD-C880F109367A 
        type: organization
        name: Platform One
        links:
        - href: <https://p1.dso.mil>
          rel: website
    components:
    - uuid: 33d8fdde-f6ab-462a-8923-e6e4446d7a10
      type: software
      title: Kyverno
      description: |
        Deployment as Kyverno as an admission controller for a Kubernetes cluster
      purpose: Admission controller for the Kubernetes API
      responsible-roles:
      - role-id: provider
        party-uuids:
        - 72134592-08C2-4A77-ABAD-C880F109367A # matches parties entry for p1
      control-implementations:
      - uuid: 5108E5FC-C45F-477B-A542-9C5611A92485
        source: https://raw.githubusercontent.com/usnistgov/oscal-content/master/nist.gov/SP800-53/rev5/json/NIST_SP-800-53_rev5_catalog.json
        description:
          Controls implemented by Kyverno for inheritance by applications
        implemented-requirements:
        - uuid: 7D019F27-294F-4759-A44F-BA6E15370ED8
          control-id: cm-4
          description: >-
            The CLI can be used in CI/CD pipelines to assist with the resource authoring process to ensure they conform to standards prior to them being deployed.
        - uuid: 91302CE7-181E-4464-9E26-2A1E42D8909F
          control-id: cm-4.1
          description: >-
            Use of auditing validationFailureAction state in a test environment would allow changes to be tested against policies without blocking development. Allowing for policies to be mirrored and enforced in production.
        - uuid: BE54EDE4-8279-4AE6-B8C3-5B68CC235E5E
          control-id: cm-6
          description: >-
            Kyverno can be configured for cluster-wide and namespaced policies for system configuration. Exceptions can be implemented to policies that will allow for explicit deviations approved by policies/configurations declared in git.
        - uuid: 6e1f05fc-3eab-45a2-9b16-d2c5acfed20b
          control-id: cm-7
          description: >-
            Kyverno can enact policies that prevent the use of specific service types (IE, LoadBalancer or NodePort)
        - uuid: C14EA5F8-3926-4BB4-BE44-B134513F143D
          control-id: cm-7.5
          description: >-
            Policies can be written to enact deny-all for workloads unless exceptions are identified
        - uuid: 69A5689A-DAA5-48F6-9953-AEF482B0FEE0
          control-id: cm-8.3
          description: >-
            Policies can be written to validate all software workloads can be verified against a signature.
        - uuid: D0CEE97B-A884-4ECB-B56E-34048148144C
          control-id: cm-8.3
          description: >-
            Policies can be written to restrict the software that can be installed by cluster users.
        - uuid: CBCB72ED-3161-4A6F-B522-FB7082E6E380
          control-id: sr-11
          description: >-
            Cluster-Wide Policies can be written to require all images be verified through signature verification.
    back-matter: 
      resources:
      - uuid: 0711df1f-d740-4e39-a25f-15cc7a017f57
        title: Kyverno
        rlinks:
        - href: https://github.com/kyverno/kyverno
      - uuid: 611ba6d8-8023-4858-b74f-957b15461ac5
        title: Big Bang Kyverno package
        rlinks:
          - href: https://repo1.dso.mil/platform-one/big-bang/apps/sandbox/kyverno
    ```

1. Push the Zarf package to an OCI registry:

    ```bash
    zarf package publish zarf-package-oscal-example-amd64-v0.0.1.tar.zst oci://<registry>/<account>
    ```
