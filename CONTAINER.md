# 1. Build the Docker Image 🏗️

First, clone the `kubectl-ai` repository and build the Docker image from the source code.

```bash
git clone https://github.com/GoogleCloudPlatform/kubectl-ai.git
cd kubectl-ai
docker build -t kubectl-ai:latest -f images/kubectl-ai/Dockerfile .
```

# 2. Provide Credentials to the Container 

The `kubectl-ai` container needs credentials to communicate with your Kubernetes cluster and Google Cloud services. You provide these by mounting local configuration files into the container when you run it.

#### **Kubernetes Cluster Access (`kubeconfig`)**

`kubectl-ai` uses a **`kubeconfig`** file to find and authenticate with your Kubernetes cluster. This file is typically located at `~/.kube/config`.

  * To generate this file for a Google Kubernetes Engine (GKE) cluster, run the following `gcloud` command:

    ```bash
    gcloud container clusters get-credentials <cluster-name> --location <location>
    ```

  * You'll later mount this directory into the container using the `-v` flag, like this: `-v ~/.kube:/root/.kube`.

    > **Note:** You can also specify a custom path to your config file using the `KUBECONFIG` environment variable.

#### **Google Cloud API Access (ADC)**

To use Google Cloud services like the Vertex AI model provider, `kubectl-ai` requires **Application Default Credentials (ADC)**.

  * On Google Cloud Shell or a GCE VM, these credentials are provided automatically.

  * On your local machine, generate them by running:

    ```bash
    gcloud auth application-default login
    ```

  * This command saves credentials to `~/.config/gcloud`. You'll also mount this directory into the container: `-v ~/.config/gcloud:/root/.config/gcloud`.

# 3. Running the Container

This example shows how to run `kubectl-ai` with a web interface, mounting all necessary credentials and providing a Gemini API key.

```bash
export GEMINI_API_KEY="your_api_key_here"
docker run --rm -it -p 8080:8080 -v ~/.kube:/root/.kube -e GEMINI_API_KEY kubectl-ai:latest --ui-listen-address 0.0.0.0:8080 --ui-type web
```

Alternativley with the default terminal ui:

```bash
export GEMINI_API_KEY="your_api_key_here"
docker run --rm -it -v ~/.kube:/root/.kube -e GEMINI_API_KEY kubectl-ai:latest --ui-listen-address 0.0.0.0:8080 
```
