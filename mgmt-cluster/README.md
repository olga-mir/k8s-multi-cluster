# Why is this here?

This folder contains files needed to bootstrap management cluster on AWS. It is at first created as a workload CAPI cluster from a temporary `kind` management cluster.

It is probably easier to install flux on the `kind` cluster that will apply this, just in the same way as permanent `mgmt` cluster applies the workload cluster. I am a bit tired of that turtle all the way down thing, so I'll leave for now whatever works.

There is a bit of an inconvenience that flux can't be installed with helm chart, so it's either terraform or run flux command: https://github.com/fluxcd/flux2/issues/1641
