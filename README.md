# Notification Manager
Notification Manager manages notifications in multi-tenant K8s environment. It receives alerts or notifications from different senders and then send notifications to various tenant receivers based on alerts/notifications' tenant label like "namespace". 

Supported senders includes:
- Prometheus Alertmanager
- Custom sender (Coming soon)

Supported receivers includes:
- Email
- [WeCom](https://work.weixin.qq.com/)
- Slack 
- Webhook 
- DingTalk

## Architecture
Notification Manager uses CRDs to store notification configs like email, wechat and slack. It also includes an operator to create and reconcile NotificationManager CRD which watches all notification config CRDs, updates notification settings accordingly and sends notifications to users.

![Architecture](docs/images/architecture.png)

## Integration with Alertmanager
Notification Manager could receive webhook notifications from Alertmanager and then send notifications to users in a multi-tenancy way.

![Notification Manager](docs/images/notification-manager.png)

## CustomResourceDefinitions
Notification Manager uses the following CRDs to define the desired alerts/notifications webhook and receiver configs:
- NotificationManager: Defines the desired alerts/notification webhook deployment. The Notification Manager Operator ensures a deployment meeting the resource requirements is running.
- Config: Defines the dingtalk, email, slack, webhook, wechat configs. 
- Receiver: Define dingtalk, email, slack, webhook, wechat receivers.

The relationship between receivers and configs can be demonstrated as below:

![Receivers & Configs](docs/images/receivers_configs.png)

Receiver CRDs like EmailReceiver, WechatReceiver, SlackReceiver and WebhookReceiver can be categorized into 2 types `global` and `tenant` by label like `type = global`, `type = tenant` :
- A global EmailReceiver receives all alerts and then send notifications regardless tenant info(user or namespace).
- A tenant EmailReceiver receives alerts with specified tenant label like `user` or `namespace` 

Usually alerts received from Alertmanager contains a `namespace` label, Notification Manager uses this label to decide which receiver to use for sending notifications:
- For D3os, Notification Manager will try to find workspace `user` in that `namespace`'s rolebinding and then find receivers with `user = xxx` label.
- For other Kubernetes cluster, Notification Manager will try to find receivers with `namespace = xxx` label. 

For alerts without a `namespace` label, for example alerts of node or kubelet, user can set up a receiver with `type = global` label to receive alerts without a `namespace` label. A global receiver sends notifications for all alerts received regardless any label. A global receiver is usually set for an admin role.

Config CRDs can be categorized into 2 types `tenant` and `default` by label like `type = tenant`, `type = default`:
- Tenant EmailConfig is to be selected by a tenant EmailReceiver which means each tenant can have his own EmailConfig. 
- If no EmailConfig selector is configured in a EmailReceiver, then this EmailReceiver will try to find a `default` EmailConfig. Usually admin will set a global default config.

A receiver could be configured without xxxConfigSelector, in which case Notification Manager will try to find a default xxxConfigSelector with `type = default` label, for example:
- A global EmailReceiver with `type = global` label should always use the default EmailConfig which means emailConfigSelector needn't to be configured for a global EmailReceiver and one default EmailConfig with `type = default` label needs to be configured for all global EmailReceivers.  
- Usually a tenant EmailReceiver with `type = tenant` label could have its own tenant emailConfigSelector to find its tenant EmailConfig with `type = tenant` label.
- A tenant EmailReceiver with `type = tenant` label can also be configured without a emailConfigSelector, in which case Notification Manager will try to find the default EmailConfig with `type = default` label for this tenant EmailReceiver.

## QuickStart

Deploy CRDs and the Notification Manager Operator:

```shell
kubectl apply -f https://raw.githubusercontent.com/d3os/notification-manager/release-1.0/config/bundle.yaml
```

### Deploy Notification Manager in D3os (Uses `workspace` to distinguish each tenant user):

#### Deploy Notification Manager
```shell
cat <<EOF | kubectl apply -f -
apiVersion: notification.d3os.io/v2beta1
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  replicas: 1
  resources:
    limits:
      cpu: 500m
      memory: 1Gi
    requests:
      cpu: 100m
      memory: 20Mi
  image: d3os/notification-manager:v1.0.0
  imagePullPolicy: IfNotPresent
  serviceAccountName: notification-manager-sa
  portName: webhook
  defaultConfigSelector:
    matchLabels:
      type: default
  receivers:
    tenantKey: user
    globalReceiverSelector:
      matchLabels:
        type: global
    tenantReceiverSelector:
      matchLabels:
        type: tenant
    options:
      global:
        templateFile:
        - /etc/notification-manager/template
      email:
        notificationTimeout: 5
        deliveryType: bulk
        maxEmailReceivers: 200
      wechat:
        notificationTimeout: 5
      slack:
        notificationTimeout: 5
  volumeMounts:
  - mountPath: /etc/notification-manager/
    name: template
  volumes:
  - configMap:
      defaultMode: 420
      name: template
    name: template
EOF
```

#### Deploy the default EmailConfig and global EmailReceiver
```
cat <<EOF | kubectl apply -f -
apiVersion: notification.d3os.io/v2beta1
kind: Config
metadata:
  labels:
    app: notification-manager
    type: default
  name: default-email-config
spec:
  email:
    authPassword:
      key: password
      name: default-email-secret
    authUsername: sender1 
    from: sender1@xyz.com
    requireTLS: true
    smartHost:
      host: imap.xyz.com
      port: 25
---
apiVersion: notification.d3os.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: global
  name: global-email-receiver
spec:
  email:
    enabled: true
    # emailConfigSelector needn't to be configured for a global receiver
    to:
    - receiver1@xyz.com
    - receiver2@xyz.com
---
apiVersion: v1
data:
  password: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-email-secret
  namespace: d3os-monitoring-system
type: Opaque
EOF
```

#### Deploy a tenant EmailConfig and a EmailReceiver
```
cat <<EOF | kubectl apply -f -
apiVersion: notification.d3os.io/v2beta1
kind: Config
metadata:
  labels:
    app: notification-manager
    type: tenant
    user: user1 
  name: user1-email-config
spec:
  email:
    authPassword:
      key: password
      name: default-email-secret
    authUsername: sender1 
    from: sender1@xyz.com
    requireTLS: true
    smartHost:
      host: imap.xyz.com
      port: 25
---
apiVersion: notification.d3os.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: tenant
    user: user1
  name: user1-email-receiver
spec:
  email:
    # This emailConfigSelector could be omitted in which case a defalut EmailConfig should be configured
    emailConfigSelector:
      matchLabels:
        type: tenant
        user: user1 
    to:
    - receiver1@xyz.com
    - receiver2@xyz.com
---
apiVersion: v1
data:
  password: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-email-secret
  namespace: d3os-monitoring-system
type: Opaque
EOF
```

#### Deploy the default WechatConfig and global WechatReceivers

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.d3os.io/v2beta1
kind: Config
metadata:
  name: default-wechat-config
  labels:
    app: notification-manager
    type: default
spec:
  wechat:
    wechatApiUrl: < wechat-api-url >
    wechatApiSecret:
      key: wechat
      name: < wechat-api-secret >
    wechatApiCorpId: < wechat-api-corp-id >
    wechatApiAgentId: < wechat-api-agent-id >
---
apiVersion: notification.d3os.io/v2beta1
kind: Receiver
metadata:
  name: global-wechat-receiver
  labels:
    app: notification-manager
    type: global 
spec:
  wechat:
    # wechatConfigSelector needn't to be configured for a global receiver
    # optional
    # One of toUser, toParty, toParty should be specified.
    toUser: 
      - user1
      - user2
    toParty: 
      - party1
      - party2
    toTag:
      - tag1
      - tag2
---
apiVersion: v1
data:
  wechat: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: < wechat-api-secret >
  namespace: d3os-monitoring-system
type: Opaque
EOF
```
> - wechatApiAgentId is the id of app which sends messages to user in your Wechat Work.
> - wechatApiSecret is the secret of this app.
> - You can get these two parameters in App Management of your Wechat Work. 
> - Note that any user, party or tag who wants to receive notifications must be in the allowed users list of this app.

#### Deploy the default SlackConfig and global SlackReceiver

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.d3os.io/v2beta1
kind: Config
metadata:
  name: default-slack-config
  labels:
    app: notification-manager
    type: default
spec:
  slack:
    slackTokenSecret: 
      key: token
      name: < slack-token-secret >
---
apiVersion: notification.d3os.io/v2beta1
kind: Receiver
metadata:
  name: global-slack-receiver
  labels:
    app: notification-manager
    type: global
spec:
  slack:
    # slackConfigSelector needn't to be configured for a global receiver
    channels: 
      - channel1
      - channel2
---
apiVersion: v1
data:
  token: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: < slack-token-secret >
  namespace: d3os-monitoring-system
type: Opaque
EOF
```
> Slack token is the OAuth Access Token or Bot User OAuth Access Token when you create a Slack app. This app must have the scope chat:write. The user who creates the app or bot user must be in the channel which you want to send notification to.

#### Deploy the default WebhookConfig and global WebhookReceiver

```
cat <<EOF | kubectl apply -f -
apiVersion: v1
data:
  ca: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJzekNDQVZpZ0F3SUJBZ0lRWHNmaU9QTUdNVXZnVkhoNTgyV1BoREFLQmdncWhrak9QUVFEQWpBaE1STXcKRVFZRFZRUUxFd3ByZFdKbGMzQm9aWEpsTVFvd0NBWURWUVFEREFFcU1DQVhEVEl3TVRBeE5EQXpORE0xT0ZvWQpEekl6TVRNd01USTBNRE16TVRFMVdqQWhNUk13RVFZRFZRUUxFd3ByZFdKbGMzQm9aWEpsTVFvd0NBWURWUVFECkRBRXFNRmt3RXdZSEtvWkl6ajBDQVFZSUtvWkl6ajBEQVFjRFFnQUVNK0pSdzBSUjZJa2RueDB1U3FnSUtSRG8KdGErMzNMSWtRektHc1dWVzNmcStjQnk0Q3duVGR5aHN1SnIycVh0YVNXeVd1ekJIWENqTWYyTllSZG9KK2FOdwpNRzR3RGdZRFZSMFBBUUgvQkFRREFnR21NQThHQTFVZEpRUUlNQVlHQkZVZEpRQXdEd1lEVlIwVEFRSC9CQVV3CkF3RUIvekFwQmdOVkhRNEVJZ1FnYnU4R3o4bmlKNUo2SnI4ZVVDUW5YR2ZMSUhNOUhVcnRTcnBLdUYzTVhlOHcKRHdZRFZSMFJCQWd3Qm9jRWZ3QUFBVEFLQmdncWhrak9QUVFEQWdOSkFEQkdBaUVBclI4ZC9vaE5aRm81dEsvMwphMEJHTXRsQTBjZHh5bldWenBQZXY3Q05qVDBDSVFEQjFzN2h6dXZPM1dMWis4MG9XWFFiSDR3bE83em9MVUhQCnJGVTF3ZWtSb0E9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
  cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJ6akNDQVhTZ0F3SUJBZ0lSQUk4VjkwMDViZGJNc2psUzYxRTliTm93Q2dZSUtvWkl6ajBFQXdJd0lURVQKTUJFR0ExVUVDeE1LYTNWaVpYTndhR1Z5WlRFS01BZ0dBMVVFQXd3QktqQWdGdzB5TURFd01UUXdNek00TlRoYQpHQTh5TXpFek1ERXlOREF6TWpZeE5Wb3dJVEVUTUJFR0ExVUVDaE1LYTNWaVpYTndhR1Z5WlRFS01BZ0dBMVVFCkF3d0JLakJaTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEEwSUFCTGwxS3MyTlJueXhmUDZGQzYzTHhobWoKZ2RRTlB1MDlLKzIwZmdkM3Q3NW9GVXdDSzkrSXNlaHRTRzlnSzhSNWhiejBoZ082RGZoM0hyQ3RCMm1ZS1RpagpnWW93Z1ljd0RnWURWUjBQQVFIL0JBUURBZ0dtTUE4R0ExVWRKUVFJTUFZR0JGVWRKUUF3REFZRFZSMFRBUUgvCkJBSXdBREFwQmdOVkhRNEVJZ1FnUnc5ZXBQN1BMODhtSHBXNzh3ekJtTFBqMkhqMTZYa1pJdFJub0dPK3VUMHcKS3dZRFZSMGpCQ1F3SW9BZ1Q1Z09zSmQrajdzY2NpY3RXM0JINjVpM2owb3FrSGdaQ2gvMDVzYW5kNWN3Q2dZSQpLb1pJemowRUF3SURTQUF3UlFJaEFQT2hJUjRnQ0wxUTdCT1Y2cXNYUWIyTjhsanZzTjhYTmxzY1FsVkhsRlE4CkFpQndaWlphWGMyeC9CVEd0alhnU3pHaStTbEVVTDE3SUVaZmdZYjNkQ2tweVE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
  key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSVBpMEpMYnlUMjdNczl2RWJ4WHNCckhlYjAyZ3VLY2NNVHRQSWI5RXRyZXFvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFdVhVcXpZMUdmTEY4L29VTHJjdkdHYU9CMUEwKzdUMHI3YlIrQjNlM3ZtZ1ZUQUlyMzRpeAo2RzFJYjJBcnhIbUZ2UFNHQTdvTitIY2VzSzBIYVpncE9BPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
  password: ZmYwZDM4YWItN2IzMC00ODE1LWI5OTMtMjQwYTc3YjQwZmMw
  bearer: ZXlKaGJHY2lPaUpJVXpJMU5pSXNJblI1Y0NJNklrcFhWQ0o5LmV5SjFjMlZ5SWpvaU5XSmlZemxrT0RFdFpqYzJNUzAwWVdFNUxXSmlNekl0WkRCaU9UVmtaR05rTXpkbUlpd2laWGh3SWpveE9UWXlOalUxT0RjNUxDSnBZWFFpT2pFMk1ESTJOVFU0Tnprc0ltbHpjeUk2SW10MVltVnpjR2hsY21VaUxDSnVZbVlpT2pFMk1ESTJOVFU0TnpsOS40Rk4wS3FIRF91Q1AtRmFIMmFpT3ZPUjFsY2wtVjFyS0Z4d2RQXzNuRmY0
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-webhook-secret
  namespace: d3os-monitoring-system
type: Opaque

---
apiVersion: notification.d3os.io/v2beta1
kind: Config
metadata:
  name: default-webhook-config
  labels:
    app: notification-manager
    type: default

---
apiVersion: notification.d3os.io/v2beta1
kind: Receiver
metadata:
  name: global-webhook-receiver
  labels:
    app: notification-manager
    type: global
spec:
  webhook:
    url: http://127.0.0.1:8080/
    httpConfig: 
      bearerToken
        key: password
        name: default-webhook-secret
      tlsConfig:
        rootCA:
          key: ca
          name: default-webhook-secret
        clientCertificate:
          cert:
            key: cert
            name: default-webhook-secret
          key:
            key: key
            name: default-webhook-secret
        insecureSkipVerify: false
EOF
```

> - The `rootCA` is the server root certificate.
> - The `certificate` is the clientCertificate of client.
> - The format of bearerToken is `Authorization <bearerToken>`.

#### Deploy the default DingTalkConfig and a global DingTalkReceiver

```
cat <<EOF | kubectl apply -f -
apiVersion: v1
data:
  appkey: ZGluZ2Jla3UxR2enAyeHQ=
  appsecret: dnRFNWt2RWppOWdiZF9x
  webhook: aHR0cHM6Ly9vYXBpLmRpbmd0YWxrLmNvbS9yb2JvdC9zZW5kP2FjY2Vzc190b2tlbj0zNjUxO
  secret: U0VDZjJiMTkyOGUwOGY5ZjM4YzIwMmZGNiN2VhMjk1MTMyNDI0YTgxMDljMjFkYzYwNGU3MDkzNQ==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-dingtalk-secret
  namespace: d3os-monitoring-system
type: Opaque

---
apiVersion: notification.d3os.io/v2beta1
kind: Config
metadata:
  name: default-dingtalk-config
  labels:
    app: notification-manager
    type: default
spec:
  dingtalk:
    conversation:
      appkey: 
        key: appkey
        name: default-dingtalk-secret
      appsecret:
        key: appsecret
        name: default-dingtalk-secret

---
apiVersion: notification.d3os.io/v2beta1
kind: Receiver
metadata:
  name: global-dingtalk-receiver
  labels:
    app: notification-manager
    type: global
spec:
  dingtalk:
    conversation:
      chatids: 
        - chat1
        - chat2
    chatbot:
      webhook:
        key: webhook
        name: default-dingtalk-secret
      keywords: 
      - d3os
      secret:
        key: secret
        name: default-dingtalk-secret
EOF
```

> - DingTalkReceiver can both send messages to `conversation` and `chatbot`.
> - If you want to send messages to conversation, the application used to send messages to conversation must have the authority `Enterprise conversation`, and the IP which notification manager used to send messages must be in the white list of the application. Usually, it is the IP of Kubernetes nodes, you can simply add all Kubernetes nodes to the white list.
> - The `appkey` is the key of the application, the `appsecret` is the secret of the application.
> - The `chatids` is the id of the conversation, it can only be obtained from the response of [creating conversation](https://ding-doc.dingtalk.com/document#/org-dev-guide/create-chat).
> - The `webhook` is the URL of a chatbot, the `keywords` is the keywords of a chatbot, The `secret` is the secret of chatbot, you can get them in the setting page of chatbot.

### Deploy Notification Manager in any other Kubernetes cluster (Uses `namespace` to distinguish each tenant user):

Deploying Notification Manager in Kubernetes is similar to deploying it in D3os, the differences are:

Firstly, change the `tenantKey` to `namespace` like this.

```
apiVersion: notification.d3os.io/v2beta1
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  receivers:
    tenantKey: namespace
```

Secondly, change the label of receiver and config from `user: ${user}` to `namespace: ${namespace}` like this.

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.d3os.io/v2beta1
kind: Config
metadata:
  labels:
    app: notification-manager
    type: tenant
    namespace: default
  name: user1-email-config
spec:
  email:
    authPassword:
      key: password
      name: user1-email-secret
    authUsername: sender1 
    from: sender1@xyz.com
    requireTLS: true
    smartHost:
      host: imap.xyz.com
      port: 25
---
apiVersion: notification.d3os.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: tenant
    namespace: default
  name: user1-email-receiver
spec:
  email:
    emailConfigSelector:
      matchLabels:
        type: tenant
    to:
    - receiver3@xyz.com
    - receiver4@xyz.com
EOF
```

### Notification filter

A receiver can filter alerts by setting a label selector, only alerts that match the label selector will be sent to this receiver.
Here is a sample, this receiver will only receive alerts from auditing.

```
apiVersion: notification.d3os.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: global
  name: global-email-receiver
spec:
  email:
    to:
    - receiver1@xyz.com
    - receiver2@xyz.com
    alertSelector:
      matchLabels:
        alerttype: auditing
```

### Customize template

You can customize the message format by customizing the template. You need to create a template file include the template that you customized, and mount it to `NotificationManager`. Then you can change the template to the template which you defined.

It can set a global template, or set a template for each type of receivers. If the template of the receiver does not set, it will use the global template. If the global template does not set too, it will use the default template. The default template looks like below:

```shell
cat <<EOF | kubectl apply -f -
apiVersion: notification.d3os.io/v2beta1
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  receivers:
    options:
      global:
        template: nm.default.text
      email:
        subjectTemplate:  nm.default.text
        template: nm.default.html
      wechat:
        template: nm.default.text
      slack:
        template: nm.default.text
      webhook:
        template: webhook.default.message
      dingtalk:
        template: nm.default.text
  volumeMounts:
  - mountPath: /etc/notification-manager/
    name: template
  volumes:
  - configMap:
      defaultMode: 420
      name: template
    name: template
EOF
```

Here is the template `nm.default.text`. For more information about templates, you can see [here](https://prometheus.io/docs/alerting/latest/notifications/).

```
    {{ define "nm.default.subject" }}{{ .Alerts | len }} alert{{ if gt (len .Alerts) 1 }}s{{ end }} for {{ range .GroupLabels.SortedPairs }} {{ .Name }}={{ .Value }} {{ end }}
    {{- end }}

    {{ define "__nm_alert_list" }}{{ range . }}Labels:
    {{ range .Labels.SortedPairs }}{{ if ne .Name "runbook_url" }}- {{ .Name }} = {{ .Value }}{{ end }}
    {{ end }}Annotations:
    {{ range .Annotations.SortedPairs }}{{ if ne .Name "runbook_url"}}- {{ .Name }} = {{ .Value }}{{ end }}
    {{ end }}
    {{ end }}{{ end }}

    {{ define "nm.default.text" }}{{ template "nm.default.subject" . }}
    {{ if gt (len .Alerts.Firing) 0 -}}
    Alerts Firing:
    {{ template "__nm_alert_list" .Alerts.Firing }}
    {{- end }}
    {{ if gt (len .Alerts.Resolved) 0 -}}
    Alerts Resolved:
    {{ template "__nm_alert_list" .Alerts.Resolved }}
    {{- end }}
    {{- end }}
```

### Config Prometheus Alertmanager to send alerts to Notification Manager
Notification Manager use port `19093` and API path `/api/v2/alerts` to receive alerts sending from Prometheus Alertmanager.
To receive Alertmanager alerts, add webhook config like below to the `receivers` section of Alertmanager configuration file:

```shell
    "receivers":
     - "name": "notification-manager"
       "webhook_configs":
       - "url": "http://notification-manager-svc.d3os-monitoring-system.svc:19093/api/v2/alerts"
```

## Update

There are some breaking changes in v1.0.0 :

- All config crds are aggregated into a crd named `Config`.
- All receivers crds are aggregated into a crd named `Receiver`.
- Now the `Config`, `Receiver`, and `NotificationManager` are cluster scoped crd.
- The `NotificationManager` crd add a property named `defaultSecretNamespace` which defines the default namespace to which notification manager secrets belong.
- Now the namespace of the secret can be specified in `SecretKeySelector` like this. 
  If the `namespace` of `SecretKeySelector` has be set, notification manager will get the secret in this namespace, 
  else, notification manager will get the secret in the `defaultSecretNamespace`,
  if the `defaultSecretNamespace` does not set, will get the secret from the namespace of notification manager operator.

```yaml
    kind: Config
    metadata:
      labels:
        type: tenant
        namespace: default
      name: user1-email-config
    spec:
      email:
        authPassword:
          key: password
          name: user1-email-secret
          namespace: d3os-monitoring-system
```

- Move the configuration of DingTalk chatbot from dingtalk config to dingtalk receiver.
- Move the chatid of DingTalk conversation from dingtalk config to dingtalk receiver.
- Now the `chatid` of DingTalk conversation is an array types, and renamed to `chatids`.
- Now the `port` of email `smartHost` is an integer type.
- Now the `channel` fo slack is an array types, and renamed to `channels`.
- Move the configuration of webhook from webhook config to webhook receiver.
- Now the `toUser`, `toParty`, `toTag` of wechat receiver are array type.

### Steps to migrate crds from v0.x to v1.0

You can update the v0.x to the v1.0 by following this.

Firstly, backup the old crds and converts to new crds.

```shell
curl -o update.sh https://raw.githubusercontent.com/d3os/notification-manager/release-1.0/config/update/update.sh && sh update.sh
```

>This command will generate two directories, backup and crds. The `backup` directory store the old crds, and the `crds` directory store the new crds

Secondly, delete old crds.

```shell
kubectl delete --ignore-not-found=true crd notificationmanagers.notification.d3os.io
kubectl delete --ignore-not-found=true crd dingtalkconfigs.notification.d3os.io
kubectl delete --ignore-not-found=true crd dingtalkreceivers.notification.d3os.io
kubectl delete --ignore-not-found=true crd emailconfigs.notification.d3os.io
kubectl delete --ignore-not-found=true crd emailreceivers.notification.d3os.io
kubectl delete --ignore-not-found=true crd slackconfigs.notification.d3os.io
kubectl delete --ignore-not-found=true crd slackreceivers.notification.d3os.io
kubectl delete --ignore-not-found=true crd webhookconfigs.notification.d3os.io
kubectl delete --ignore-not-found=true crd webhookreceivers.notification.d3os.io
kubectl delete --ignore-not-found=true crd wechatconfigs.notification.d3os.io
kubectl delete --ignore-not-found=true crd wechatreceivers.notification.d3os.io
```

Thirdly, deploy the notification-manager of the latest version.

```shell
kubectl apply -f https://raw.githubusercontent.com/d3os/notification-manager/master/config/bundle.yaml
```

Finally, deploy configs and receivers.

```shell
kubectl apply -f crds/
```

## Development

```
# Build notification-manager-operator and notification-manager docker images
make build 
# Push built docker images to docker registry
make push
```
