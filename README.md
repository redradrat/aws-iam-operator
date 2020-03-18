# AWS IAM Operator

An operator that enables AWS IAM management via Kubernetes custom resources.

## Role

The Role resource abstracts an AWS IAM Role. 

Setting an `assumeRolePolicy` is **mandatory**.
Creating a `ServiceAccount` resource is possible via `createServiceAccount`. The created ServiceAccount includes the EKS OIDC annotation.

```yaml
apiVersion: aws-iam.redradrat.xyz/v1beta1
kind: Role
metadata:
  name: role-sample
  namespace: default
spec:
  assumeRolePolicy:
    - effect: "Allow"
      principal:
        "Federated": "blabla"
      actions:
        - "sts:AssumeRoleWithWebIdentity"
      conditions:
        - "blabla": "system:serviceaccount:kube-system:aws-cluster-autoscaler"
  createServiceAccount: true
```

Resulting `ServiceAccount`:
```yaml
❯ k get sa role-sample -o yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::0000000000:role/role-sample
  creationTimestamp: "2020-02-30T00:25:61Z"
  name: role-sample
  namespace: default
  ownerReferences:
  - apiVersion: aws-iam.redradrat.xyz/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: Role
    name: role-sample
    uid: ...
```

## Policy

The Policy resource abstracts an AWS IAM Policy.

For `conditions`, please check ![here](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_condition_operators.html) for valid Operators. For the comparison, only String-type values are allowed as comparison values. For keys please check out https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_condition-keys.html

```yaml
apiVersion: aws-iam.redradrat.xyz/v1beta1
kind: Policy
metadata:
  name: policy-sample
spec:
  statement:
    - sid: someid
      effect: "Allow"
      principal:
      actions:
        - "xxxx:DescribeSomething"
      resources:
        - "*"
      conditions:
        "StringEquals":
          "aws:SourceIp": "172.0.0.1"
```

## PolicyAssignment