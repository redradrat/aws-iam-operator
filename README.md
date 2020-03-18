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

## Policy

The Policy resource abstracts an AWS IAM Polcy, including its ``.

```
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