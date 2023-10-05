# RancherProjector
***
RancherProjector is a short program, meant to be run in the Rancher Management Cluster, that will gather the Rancher Projects defined and post them to a Rancher-Selector Endpoint, deployed in a downstream cluster.
Obviously, RancherProjector, needs RancherSelector to work:
https://github.com/wrkode/rancher-selector

## information

RancherProjector will watch the Rancher Management Cluster Projects.
On event such as creation/update/deletion of Projects and Projects Annotations, RancherProjector will hit RancherSelector API on the downstream cluster with POST/Delete requests.
The reasoning behind this is that Rancher Projects do not exist as CRD/Objects in the Rancher Downstream clusters. Therefor (at the time of this writing) there's no way to use the user-defined annotations defined at Project creation.
The counterpart of RancherProjector, RancherSelector, will create a ConfigMap named ```rancher-data``` in the downstream cluster, in Namespace ```kube-system```. this ConfigMap will contain all projects and annotations of all projects of the downstream cluster.

RancherProjector
## Usage
**Important**: make sure that RancherSelector is already deployed in all downstream clusters. 
- clone this repository and ```cd``` into the root directory.
- create a non-scoped Rancher API Bearer Token.
- create a secret in the kube-system namespace named ```rancher-projector-secret``` with key:value ```token=<yourBearerToken>```.
- Adjust file ```deployment.yaml``` as required.
- Deploy RancherProjector wit ```kubectl apply -f deployment.yaml```.
- Check in the downstream cluster namespace ```kube-system``` if ConfigMap ```rancher-data``` has been created.