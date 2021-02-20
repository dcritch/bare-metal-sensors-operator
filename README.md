# bare-metal-sensors-operator
An operator for monitoring temperatures of openshift/kubernetes cluster hardware.

Will deploy a custom resource of [bare-metal-sensors](https://github.com/dcritch/bare-metal-sensors).

## install

Still pretty raw, but try it out!

To deploy the operator:

~~~
git clone git@github.com:dcritch/bare-metal-sensors-operator.git
cd bare-metal-sensors-operator
make deploy IMG=quay.io/dcritch/bare-metal-sensors-operator:v0.0.1
~~~

Create a CR pointing to your existing influxdb server:

~~~
$ cat cr.yml
apiVersion: sensors.xana.du/v1alpha1
kind: BareMetalSensors
metadata:
  name: bare-metal-sensors
spec:
  influxhost: "influxdb.example.com"
  name: bare-metal-sensors
~~~

Create a new project and apply the CR:
~~~
oc new-project bare-metal-sensors
oc adm policy add-cluster-role-to-user system:openshift:scc:privileged -z bare-metal-sensors -n bare-metal-sensors # gross, sorry / todo
oc apply -f cr.yml
~~~
