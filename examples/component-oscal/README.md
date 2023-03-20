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
    description: 'Demo Zarf package composability with OSCAL documents'
    components:
      - name: oscal-data
        required: true
        description: 'Example component that has OSCAL documents'
        oscal:
            # Copy file from local filesystem
          - source: ./oscal/kyverno/oscal-component.yaml
            destination: ./kyverno/oscal-component.yaml

            # Fetch file from remote URL
          - source: https://repo1.dso.mil/big-bang/product/packages/monitoring/-/raw/main/oscal-component.yaml
            destination: ./monitoring/oscal-component.yaml
    ```

    Note the `components.oscal` field. This field is used to specify the OSCAL files that Zarf should include in this package.

    The `components.oscal.source` field is used to tell Zarf where to find and fetch an OSCAL file from. This can be either a path to a file on the local filesystem, or a remote URL that points to a raw OSCAL file for Zarf to fetch.

    The `components.oscal.destination` field is used to tell Zarf what path or directory to put the OSCAL files in the Zarf package bundle.

    You'll notice that one OSCAL file is being pulled from the current directory we're working out of:

    ```yaml
    # Copy file from local filesystem
    - source: ./oscal/kyverno/oscal-component.yaml
      destination: ./kyverno/oscal-component.yaml
    ```

    And the second OSCAL file is being pulled from a remote URL:

    ```yaml
    # Fetch file from remote URL
    - source: https://repo1.dso.mil/big-bang/product/packages/monitoring/-/raw/main/oscal-component.yaml
      destination: ./monitoring/oscal-component.yaml
    ```

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
    Monitoring      ac-6.9
    Monitoring      au-2
    Monitoring      au-3.1
    Monitoring      au-4
    Monitoring      au-5
    Monitoring      au-5.1
    Monitoring      au-5.2
    Monitoring      au-6.1
    Monitoring      au-6.3
    Monitoring      au-6.5
    Monitoring      au-6.6
    Monitoring      au-7
    Monitoring      au-7.1
    Monitoring      au-8
    Monitoring      au-9
    Monitoring      au-9.2
    Monitoring      au-9.4
    Monitoring      au-12.1
    ```

    The output shows which controls a component/application satisfies based on the OSCAL files included in the Zarf package.

    The descriptions for how each control is satisfied by a given application are long and are difficult to read in the table output in the terminal, so they've been excluded.
