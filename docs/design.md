<!-- Just because a KEP is merged does not mean it is complete or approved.
Any KEP marked as `provisional` is a working document and subject to change.
You can denote sections that are under active debate as follows:

``` <<[UNRESOLVED optional short context or usernames ]>> Stuff that is being
argued. <<[/UNRESOLVED]>> ```

When editing KEPS, aim for tightly-scoped, single-topic PRs to keep
discussions focused. If you disagree with what is already in a document, open
a new PR with suggested changes.

One KEP corresponds to one "feature" or "enhancement" for its whole
lifecycle. You do not need a new KEP to move from beta to GA, for example. If
new details emerge that belong in the KEP, edit the KEP. Once a feature has
become "implemented", major changes should get new KEPs.

The canonical place for the latest set of instructions (and the likely source
of this file) is [here](/keps/NNNN-kep-template/README.md).
**Note:** Any PRs to move a KEP to `implementable`, or significant changes
*once
it is marked `implementable`, must be approved by each of the KEP approvers.
If none of those approvers are still appropriate, then changes to that list
should be approved by the remaining approvers and/or the owning SIG (or SIG
Architecture for cross-cutting KEPs).
-->
# Design of `LNamespace`
<!-- This is the title of your KEP. Keep it short, simple, and descriptive. A
good title can help communicate what the KEP is and should be considered as
part of any review.
-->
<!-- Ensure the TOC is wrapped with <code>&lt;!-- toc --&rt;&lt;!-- /toc
--&rt;</code> tags, and then generate with `hack/update-toc.sh`.
-->
<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories (Optional)](#user-stories-optional)
    - [Story 1](#story-1)
    - [Story 2](#story-2)
  - [Notes/Constraints/Caveats (Optional)](#notesconstraintscaveats-optional)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [<code>sudo</code> model](#-model)
  - [Test Plan](#test-plan)
  - [Graduation Criteria](#graduation-criteria)
  - [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
  - [Version Skew Strategy](#version-skew-strategy)
- [Production Readiness Review Questionnaire](#production-readiness-review-questionnaire)
  - [Feature Enablement and Rollback](#feature-enablement-and-rollback)
  - [Rollout, Upgrade and Rollback Planning](#rollout-upgrade-and-rollback-planning)
  - [Monitoring Requirements](#monitoring-requirements)
  - [Dependencies](#dependencies)
  - [Scalability](#scalability)
  - [Troubleshooting](#troubleshooting)
- [Implementation History](#implementation-history)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)
- [Infrastructure Needed (Optional)](#infrastructure-needed-optional)
<!-- /toc -->

## Summary
Historically, the tenants of our GKE clusters at Loblaw Digital has been
managed via a gitops repo called `shipyard/builder`. We propose a drop in
replacement for shipyard/builder in the form of a CRD called `LNamespace`.

## Motivation
As the number of tenants increase, a GitOps flow begins to strain at its own
limitations: 
- The number of merge requests increase as your user base scales
- The power of a merge request is simultaneously its downfall; It's
_possible_ to put a governance project, but it's not a good idea to. This
creates pushback from our users ("why can't we do it like this? it'd be so
much easier.")

A clean interface (read: clean, well formed input data) is necessary for
clearly communicating what _is_ and _is not_ possible in a system. This
solution aims to replace the tenant management portion of `shipyard/builder`
to reduce the workload of shipyard maintainers.
### Goals
Drop in replacement for the functionality of shipyard/builder.

Brief requirements, expanded below:
- Access control. This sets up the whole permission model of the cluster.
  - Permission model
- Billing metadata


### Non-Goals



## Proposal
Note that we originally intended on creating a `Namespace` resource, and was
blocked by [a bug on Namespace
CRDs](https://github.com/kubernetes/kubernetes/issues/97397).

### User Stories (Optional)
<!-- Detail the things that people will be able to do if this KEP is
implemented. Include as much detail as possible so that people can understand
the "how" of the system. The goal here is to make this feel real for users
without getting bogged down.
-->

#### Story 1
`kubectl create lns`
#### Story 2

### Notes/Constraints/Caveats (Optional)
<!-- What are the caveats to the proposal? What are some important details
that didn't come across above? Go in to as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

### Risks and Mitigations
<!-- What are the risks of this proposal, and how do we mitigate? Think
broadly. For example, consider both security and how this will impact the
larger Kubernetes ecosystem.

How will security be reviewed, and by whom?

How will UX be reviewed, and by whom?

Consider including folks who also work outside the SIG or subproject.
-->

## Design Details

### Billing metadata
The user making the request should have some sort of default billing metadata
on their namespace. This data could potentially be sourced via their Google
Group data, but it's unclear how, since user -> group mapping is many:many.

The namespace billing data must be pushed to a table on bigquery -- the
table, dataset, and project names shall be provided via configuration.

The design of the billing controller is designed such that the bq can be
mocked via any engine that supports the SQL spec. For testing, we use
datadog's sqlmock.

### Permission model
With this feature, we intend to introduce the concept of sudoers into
kubernetes. With a multitenant system, an added layer complexity is getting
sudo permissions on the namespace layer. As such, we intend to design it as
such: 
- All loblaw accounts have access to a view-only permission set by
default. 
- Editing to the `normal` set of resources is given by a user list.
- Editing the `lnamespace` definition is given by a managers list.
- Editing of low level resources, in addition to all of the above permissions
is given by a sudoers list.

Normal permissions will be bound in a normal manner (a long lived
`RoleBinding`). However, sudo permissions will be bound to a group that must
be impersonated by setting the `Impersonate-Group` header on the request. The
user will have an alias set up, such that the experience will be similar
something like: `kubectl sudo edit lns namespace-sample`.
```
<<[UNRESOLVED]>> 
Unfortunately, this doesn't allow for the trick: `sudo !!`. 
<<[/UNRESOLVED]>> 
```

This means that during `lnamespace` set up, an RBAC controller will read the
User list. For each user, grant a rolebinding to edit a common list of
resources that is defined within a role.

On the other hand, sudo permissions will be granted to a group of naming
convention `<namespace-name>-sudoers`. As well, permission to impersonate
that group will be bound to the user account of each sudoer. Therefore, the
raw command that the user will need to run is as follows. Note that the
experience can be improved via aliases.
```
kubectl --as=$USER --as-group:namespace-sample-sudoers apply -f resource.yaml
```

Finally, this installer also sets up the default binding for
system:authenticated users. This provides the default user group for users at
Loblaw, which includes viewer permissions as well as create `LNamespace`
permissions. This is set up by aggregating the labels on `view` as well as
`authenticated`, and binding `group:system:authenticated` to it.

#### Sudoer impersonation implementation details
When a sudoers list is changed, controller iterates through sudoers, and
_each_ `User` will have a ClusterRole and ClusterRoleBinding over the entire
system. This allows the user to impersonate themselves, and _only_
themselves. This is required due to a technical limitation of requiring
impersonate user headers while using impersonate group headers. Each
ClusterRole and ClusterRoleBinding will have their OwnerReferences set to the
`LNamespace` resources that depend on them. This allows for GC when the
LNamespaces are cleaned up.

- ClusterRole - docs/resource-samples/self-impersonator-cluster-role.yaml
- ClusterRoleBinding - docs/resource-samples/self-impersonator-cluster-role-binding.yaml


A `LNamespace` will have _one_ additional ClusterRole and ClusterRoleBinding
that denotes the sudoer group. The `ClusterRoleBinding.subjects` shall map
with the sudoers list in the `LNamespace` resource. This allows the user to
impersonate the sudoer group when they require elevated permissions.

- ClusterRole - docs/resource-samples/sudoer-impersonator-cluster-role.yaml
- ClusterRoleBinding - docs/resource-samples/sudoer-impersonator-cluster-role-binding.yaml

---

**Why are the above `ClusterRoles`, and not `Roles`?**

Users and groups cannot be namespaced -- they're cluster wide resources.
Therefore, the impersonation steps must be done on a cluster wide scale
before operating on namespace aware resources.

---


Finally, a sudoer group shall have `cluster-admin` permissions inside of the
namespace, as well as edit permissions on its specific `LNamespace` resource.
. This requires a rolebinding to bind
the `cluster-admin` permission inside of the namespace, as well as a cluster
role and cluster role binding to bind editor permissions to _only_ this
`LNamespace`:
- RoleBinding - docs/resource-samples/sudoer-group-cluster-admin-role-binding.yaml
- ClusterRole - docs/resource-samples/sudoer-lnamespace-edit-cluster-role.yaml
- ClusterRoleBinding - docs/resource-samples/sudoer-lnamespace-edit-cluster-role-binding.yaml
#### Manager RBAC
Managers are permanently bound with the ability to edit their `LNamespaces`,
but will have no other permissions by default.
#### User RBAC
User RBAC will be created and bound for the lifetime of the user entry. This
means that the user will have access to their user permissions without
needing to impersonate another group. However, it's important to consider
that users may soon desire different types.

---
If users were to desire different types, they will need to be familiar with
the concept of Roles and RoleBindings. The user will be responsible for
maintaining their Roles and RoleBindings with their sudoer permissions, and
the custom users shall simply be removed from the Users list of the
`LNamespace`.

---

Users should be afforded all permissions. However there are a few permissions
that should not be afforded. Notably, `Namespace` editing should not be
provided to a user except in very specific circumstances. I have vetted the
`admin` role that is provided as part of bootstrapping, and it does not
contain namespace edit permissions. However, the downside with this approach
is that it is unclear how to differentiate between `cluster-admin` and
`admin`, as there is no comprehensive list of what the `cluster-admin` role
contains. In addition, `admin` is a construct controlled by k8s, and so the
permissions granted by admin may change.

Specifically, `admin` is built via aggregation of label selectors. To get a
detailed view of what the admin role entails, run the command `kubectl get
clusterrole -l rbac.authorization.k8s.io/aggregate-to-admin="true"`. The results of this command will have to be reviewed every GKE version. At the latest review of this:
```
NAME                        AGE
cert-manager-edit           12d
cert-manager-view           12d
edit                        12d
system:aggregate-to-admin   12d
```
All of these permissions do not contain `namespace` editing permissions. 

User permissions will need to be vetted every GKE version. There will need to
be a "last vetted" version of `kubectl api-resources`, and each new version
will compared against the last vetted version.

#### RBAC Cleanup
The default controller logic for garbage collection is only invoked when the
parent resource is deleted from the server. However, we require that sudoer
and user RBAC resources be cleaned up when their usernames are removed from
their respective lists.

The self impersonator CR/CRB in particular is a shared CR/CRB across multiple
namespaces. The requirements for these resources are that: delete iff no
`LNamespace` resources contain that user in the sudoers list.

`controllerbuilder.Owns` by default only updates the ownerResource with
`controller: true`. Therefore, we watch and notify all owning resources with
the correct labels instead of just `controller: true`.

If a lns is deleted, the GC will clean up the owner reference on the CR/CRB
resources. The GC will also clean up any resources if after owner resource
cleanup `len(ownerReferences) == 0`.

Therefore, RBAC controller needs to account for edit logic only. On edit, the
RBAC controller needs to get all owned resources, and compare to the current
sudoers list. The algo:
- The RBAC controller will list all CR/CRB.
- For each CR/CRB, if there exists an owner reference pointing to the `lns`:
  - Check if sudoers list contains CR/CRB subject.User. If yes, leave it.
    - If no, trigger the GC by setting the UUID of the owner reference to `mark-for-deletion`.

The GC shall remove the ownerReference, and delete the resource entirely if there are no other owners.

### Test Plan
E2E testing shall be completed via a test user. For example:
- The tester creates `lns` resource with the User/Sudoer list set to Name `richard-song-test-user` with Kind `User`
- The tester tests under that user with `--as` and `--as-group` flags. For example: `kubectl auth can-i --as=richard-song-test-user get po`


When impersonation specific attributes need to be tested, the kind cluster
will have to generate a new user. Details can be found here:
https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/#normal-user
`hack/new-user.sh` is provided for your convenience.

We encourage `package_test` style tests. This allows us to test the public
facing interfaces without reaching in and caring too much about the
implementation details.

#### Testing strategy for webhooks
Note that webhooks report back a list of json patches that need to be
completed on the requested, and the API server is the one that actually
applies the JSON patches. To lighten our test cases as much as possible, test
cases should inspect response's JSON patches list instead of actually
applying them.
### Graduation Criteria
<!--
**Note:** *Not required until targeted at a release.*
Define graduation milestones.

These may be defined in terms of API maturity, or as something else. The KEP
should keep this high-level with a focus on what signals will be looked at to
determine graduation.

Consider the following in developing the graduation criteria for this
enhancement: - [Maturity levels (`alpha`, `beta`, `stable`)][maturity-levels]
- [Deprecation policy][deprecation-policy]

Clearly define what graduation means by either linking to the [API doc
definition](https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-versioning)
or by redefining what graduation means.

In general we try to use the same stages (alpha, beta, GA), regardless of how
the functionality is accessed.

[maturity-levels]:
https://git.k8s.io/community/contributors/devel/sig-architecture/api_changes.md#alpha-beta-and-stable-versions
[deprecation-policy]:
https://kubernetes.io/docs/reference/using-api/deprecation-policy/

Below are some examples to consider, in addition to the aforementioned
[maturity levels][maturity-levels].
#### Alpha -> Beta Graduation
- Gather feedback from developers and surveys - Complete features A, B, C -
Tests are in Testgrid and linked in KEP
#### Beta -> GA Graduation
- N examples of real-world usage - N installs - More rigorous forms of
testing—e.g., downgrade tests and scalability tests - Allowing time for
feedback
**Note:** Generally we also wait at least two releases between beta and
GA/stable, because there's no opportunity for user feedback, or even bug
reports, in back-to-back releases.
#### Removing a Deprecated Flag
- Announce deprecation and support policy of the existing flag - Two versions
passed since introducing the functionality that deprecates the flag (to
address version skew) - Address feedback on usage/changed behavior, provided
on GitHub issues - Deprecate the flag
**For non-optional features moving to GA, the graduation criteria must include
[conformance tests].**

[conformance tests]:
https://git.k8s.io/community/contributors/devel/sig-architecture/conformance-tests.md
-->

### Upgrade / Downgrade Strategy
<!-- If applicable, how will the component be upgraded and downgraded? Make
sure this is in the test plan.

Consider the following in developing an upgrade/downgrade strategy for this
enhancement: - What changes (in invocations, configurations, API use, etc.)
is an existing cluster required to make on upgrade, in order to maintain
previous behavior? - What changes (in invocations, configurations, API use,
etc.) is an existing cluster required to make on upgrade, in order to make
use of the enhancement?
-->

### Version Skew Strategy
<!-- If applicable, how will the component handle version skew with other
components? What are the guarantees? Make sure this is in the test plan.

Consider the following in developing a version skew strategy for this
enhancement: - Does this enhancement involve coordinating behavior in the
control plane and in the kubelet? How does an n-2 kubelet without this
feature available behave when this feature is used? - Will any other
components on the node change? For example, changes to CSI, CRI or CNI may
require updating that component before the kubelet.
-->

## Production Readiness Review Questionnaire
<!--

Production readiness reviews are intended to ensure that features merging
into Kubernetes are observable, scalable and supportable; can be safely
operated in production environments, and can be disabled or rolled back in
the event they cause increased failures in production. See more in the PRR
KEP at
https://git.k8s.io/enhancements/keps/sig-architecture/1194-prod-readiness/README.md.

The production readiness review questionnaire must be completed and approved
for the KEP to move to `implementable` status and be included in the release.

In some cases, the questions below should also have answers in `kep.yaml`.
This is to enable automation to verify the presence of the review, and to
reduce review burden and latency.

The KEP must have a approver from the
[`prod-readiness-approvers`](http://git.k8s.io/enhancements/OWNERS_ALIASES)
team. Please reach out on the
[#prod-readiness](https://kubernetes.slack.com/archives/CPNHUMN74) channel if
you need any help or guidance.
-->

### Feature Enablement and Rollback
_This section must be completed when targeting alpha to a release._
* **How can this feature be enabled / disabled in a live cluster?**
- [ ] Feature gate (also fill in values in `kep.yaml`) - Feature gate name: -
Components depending on the feature gate: - [ ] Other - Describe the
mechanism: - Will enabling / disabling the feature require downtime of the
control plane? - Will enabling / disabling the feature require downtime or
reprovisioning of a node? (Do not assume `Dynamic Kubelet Config` feature is
enabled).
* **Does enabling the feature change any default behavior?**
Any change of default behavior may be surprising to users or break existing
automations, so be extremely careful here.
* **Can the feature be disabled once it has been enabled (i.e. can we roll back
the enablement)?** Also set `disable-supported` to `true` or `false` in
`kep.yaml`. Describe the consequences on existing workloads (e.g., if this is
a runtime feature, can it break the existing applications?).
* **What happens if we reenable the feature if it was previously rolled back?**

* **Are there any tests for feature enablement/disablement?**
The e2e framework does not currently support enabling or disabling feature
gates. However, unit tests in each component dealing with managing data,
created with and without the feature, are necessary. At the very least, think
about conversion tests if API types are being modified.
### Rollout, Upgrade and Rollback Planning
_This section must be completed when targeting beta graduation to a release._
* **How can a rollout fail? Can it impact already running workloads?**
Try to be as paranoid as possible - e.g., what if some components will
restart mid-rollout?
* **What specific metrics should inform a rollback?**

* **Were upgrade and rollback tested? Was the upgrade->downgrade->upgrade path
* tested?**
Describe manual testing that was done and the outcomes. Longer term, we may
want to require automated upgrade/rollback tests, but we are missing a bunch
of machinery and tooling and can't do that now.
* **Is the rollout accompanied by any deprecations and/or removals of features,
* APIs,
fields of API types, flags, etc.?** Even if applying deprecation policies,
they may still surprise some users.
### Monitoring Requirements
_This section must be completed when targeting beta graduation to a release._
* **How can an operator determine if the feature is in use by workloads?**
Ideally, this should be a metric. Operations against the Kubernetes API
(e.g., checking if there are objects with field X set) may be a last resort.
Avoid logs or events for this purpose.
* **What are the SLIs (Service Level Indicators) an operator can use to
* determine
the health of the service?** - [ ] Metrics - Metric name: - [Optional]
Aggregation method: - Components exposing the metric: - [ ] Other (treat as
last resort) - Details:
* **What are the reasonable SLOs (Service Level Objectives) for the above
* SLIs?**
At a high level, this usually will be in the form of "high percentile of SLI
per day <= X". It's impossible to provide comprehensive guidance, but at the
very high level (needs more precise definitions) those may be things like: -
per-day percentage of API calls finishing with 5XX errors <= 1% - 99%
percentile over day of absolute value from (job creation time minus expected
job creation time) for cron job <= 10% - 99,9% of /health requests per day
finish with 200 code
* **Are there any missing metrics that would be useful to have to improve
* observability
of this feature?** Describe the metrics themselves and the reasons why they
weren't added (e.g., cost, implementation difficulties, etc.).
### Dependencies
_This section must be completed when targeting beta graduation to a release._
* **Does this feature depend on any specific services running in the cluster?**
Think about both cluster-level services (e.g. metrics-server) as well as
node-level agents (e.g. specific version of CRI). Focus on external or
optional services that are needed. For example, if this feature depends on a
cloud provider API, or upon an external software-defined storage or network
control plane.

For each of these, fill in the following—thinking about running existing user
workloads and creating new ones, as well as about cluster-level services
(e.g. DNS): - [Dependency name] - Usage description: - Impact of its outage
on the feature: - Impact of its degraded performance or high-error rates on
the feature:
### Scalability
_For alpha, this section is encouraged: reviewers should consider these
questions and attempt to answer them._

_For beta, this section is required: reviewers must answer these questions._

_For GA, this section is required: approvers should be able to confirm the
previous answers based on experience in the field._
* **Will enabling / using this feature result in any new API calls?**
Describe them, providing: - API call type (e.g. PATCH pods) - estimated
throughput - originating component(s) (e.g. Kubelet, Feature-X-controller)
focusing mostly on: - components listing and/or watching resources they
didn't before - API calls that may be triggered by changes of some Kubernetes
resources (e.g. update of object X triggers new updates of object Y) -
periodic API calls to reconcile state (e.g. periodic fetching state,
heartbeats, leader election, etc.)
* **Will enabling / using this feature result in introducing new API types?**
Describe them, providing: - API type - Supported number of objects per
cluster - Supported number of objects per namespace (for namespace-scoped
objects)
* **Will enabling / using this feature result in any new calls to the cloud
provider?**
* **Will enabling / using this feature result in increasing size or count of
the existing API objects?** Describe them, providing: - API type(s): -
Estimated increase in size: (e.g., new annotation of size 32B) - Estimated
amount of new objects: (e.g., new Object X for every existing Pod)
* **Will enabling / using this feature result in increasing time taken by any
operations covered by [existing SLIs/SLOs]?** Think about adding additional
work or introducing new steps in between (e.g. need to do X to start a
container), etc. Please describe the details.
* **Will enabling / using this feature result in non-negligible increase of
resource usage (CPU, RAM, disk, IO, ...) in any components?** Things to keep
in mind include: additional in-memory state, additional non-trivial
computations, excessive access to disks (including increased log volume),
significant amount of data sent and/or received over network, etc. This
through this both in small and large cases, again with respect to the
[supported limits].
### Troubleshooting
The Troubleshooting section currently serves the `Playbook` role. We may
consider splitting it into a dedicated `Playbook` document (potentially with
some monitoring details). For now, we leave it here.

_This section must be completed when targeting beta graduation to a release._
* **How does this feature react if the API server and/or etcd is unavailable?**

* **What are other known failure modes?**
For each of them, fill in the following information by copying the below
template: - [Failure mode brief description] - Detection: How can it be
detected via metrics? Stated another way: how can an operator troubleshoot
without logging into a master or worker node? - Mitigations: What can be done
to stop the bleeding, especially for already running user workloads? -
Diagnostics: What are the useful log messages and their required logging
levels that could help debug the issue? Not required until feature graduated
to beta. - Testing: Are there any tests for failure mode? If not, describe
why.
* **What steps should be taken if SLOs are not being met to determine the
* problem?**
[supported limits]:
https://git.k8s.io/community//sig-scalability/configs-and-limits/thresholds.md
[existing SLIs/SLOs]:
https://git.k8s.io/community/sig-scalability/slos/slos.md#kubernetes-slisslos
## Implementation History
<!-- Major milestones in the lifecycle of a KEP should be tracked in this
section. Major milestones might include: - the `Summary` and `Motivation`
sections being merged, signaling SIG acceptance - the `Proposal` section
being merged, signaling agreement on a proposed design - the date
implementation started - the first Kubernetes release where an initial
version of the KEP was available - the version of Kubernetes where the KEP
graduated to general availability - when the KEP was retired or superseded
-->

## Drawbacks
<!-- Why should this KEP _not_ be implemented?
-->

## Alternatives
<!-- What other approaches did you consider, and why did you rule them out?
These do not need to be as detailed as the proposal, but should include
enough information to express the idea and why it was not acceptable.
-->

## Infrastructure Needed (Optional)
<!-- Use this section if you need things from the project/SIG. Examples
include a new subproject, repos requested, or GitHub details. Listing these
here allows a SIG to get the process for these resources started right away.
-->
