# apprun-dedicated-application-provisioner

さくらのクラウド AppRun 専有型のアプリケーション設定を YAML ファイルで管理し、同期するツールです。

## 特徴

- **Infrastructure as Code**: YAML ファイルでアプリケーション設定を宣言的に管理
- **plan/apply**: Terraform 風の2段階操作で安全に変更を適用
- **設定の継承**: YAML で指定していない項目は既存バージョンの設定を自動的に引き継ぎ
- **image の分離**: コンテナイメージは既存バージョンから継承（CI/CD でのデプロイと設定管理を分離）
- **インフラ管理**: クラスタ設定、AutoScalingGroup、LoadBalancer も YAML で管理可能

## インストール

### Docker（推奨）

```bash
docker pull ghcr.io/tokuhirom/apprun-dedicated-application-provisioner:latest
```

Docker での実行例:

```bash
docker run --rm \
  -e SAKURA_ACCESS_TOKEN="your-access-token-uuid" \
  -e SAKURA_ACCESS_TOKEN_SECRET="your-access-token-secret" \
  -v $(pwd)/apprun.yaml:/apprun.yaml:ro \
  ghcr.io/tokuhirom/apprun-dedicated-application-provisioner:latest \
  plan -c /apprun.yaml
```

### その他のインストール方法

<details>
<summary>GitHub Releases からダウンロード</summary>

[Releases ページ](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/releases) から、お使いの OS/アーキテクチャに合ったバイナリをダウンロードしてください。

</details>

<details>
<summary>go install</summary>

```bash
go install github.com/tokuhirom/apprun-dedicated-application-provisioner/cmd/apprun-dedicated-application-provisioner@latest
```

</details>

<details>
<summary>ソースからビルド</summary>

```bash
git clone https://github.com/tokuhirom/apprun-dedicated-application-provisioner.git
cd apprun-dedicated-application-provisioner
go build -o bin/apprun-dedicated-application-provisioner ./cmd/apprun-dedicated-application-provisioner
```

</details>

## 使い方

### 環境変数の設定

```bash
export SAKURA_ACCESS_TOKEN="your-access-token-uuid"
export SAKURA_ACCESS_TOKEN_SECRET="your-access-token-secret"
```

互換性のため、`SAKURACLOUD_ACCESS_TOKEN` / `SAKURACLOUD_ACCESS_TOKEN_SECRET` も使用可能です。

### 変更内容の確認 (plan)

```bash
apprun-dedicated-application-provisioner plan -c apprun.yaml
```

| オプション | 説明 |
|-----------|------|
| `--config`, `-c` | 設定ファイルのパス（必須） |

出力例:
```
Cluster: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

+ webapp (create)
    Create new application and version
~ api (update)
    CPU: 500 -> 1000
    Memory: 1024 -> 2048
  worker (no changes)

Plan: 1 to create, 1 to update, 1 unchanged.
```

### 変更の適用 (apply)

```bash
# バージョンの作成のみ（アクティブ化しない）
apprun-dedicated-application-provisioner apply -c apprun.yaml

# バージョンの作成とアクティブ化
apprun-dedicated-application-provisioner apply -c apprun.yaml --activate
```

| オプション | 説明 |
|-----------|------|
| `--config`, `-c` | 設定ファイルのパス（必須） |
| `--activate` | 作成/更新したバージョンをアクティブ化する |

**注意**: デフォルトでは `apply` はバージョンの作成/更新のみを行い、アクティブ化は行いません。`--activate` オプションを指定することで、作成/更新したバージョンを即座にアクティブ化できます。これにより、バージョンの作成と本番への反映を分離して管理できます。

### バージョン一覧の表示 (versions)

```bash
apprun-dedicated-application-provisioner versions -c apprun.yaml -a webapp
```

| オプション | 説明 |
|-----------|------|
| `--config`, `-c` | 設定ファイルのパス（必須） |
| `--app`, `-a` | アプリケーション名（必須） |

出力例:
```
Application: webapp (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)

VERSION  IMAGE                          CREATED              NODES  STATUS
3        nginx:1.25                     2024-01-15 10:30:00  2      active
2        nginx:1.24                     2024-01-10 09:00:00  0
1        nginx:1.23                     2024-01-05 08:00:00  0

Total: 3 versions
Active version: 3
Latest version: 3
```

### バージョン間の差分表示 (diff)

```bash
# アクティブバージョンと最新バージョンの差分を表示（デフォルト）
apprun-dedicated-application-provisioner diff -c apprun.yaml -a webapp

# 特定のバージョン間の差分を表示
apprun-dedicated-application-provisioner diff -c apprun.yaml -a webapp --from 1 --to 3
```

| オプション | 説明 |
|-----------|------|
| `--config`, `-c` | 設定ファイルのパス（必須） |
| `--app`, `-a` | アプリケーション名（必須） |
| `--from` | 比較元バージョン（デフォルト: アクティブバージョン） |
| `--to` | 比較先バージョン（デフォルト: 最新バージョン） |

出力例:
```
Application: webapp
Comparing version 2 → 3

  CPU: 500 -> 1000
  Memory: 1024 -> 2048
  Image: nginx:1.24 -> nginx:1.25
  Env add: NEW_VAR=value
  Env update: LOG_LEVEL=debug -> info
  Env remove: OLD_VAR

Note: secret env values and registryPassword cannot be compared (values not returned by API)
```

**注意**: `secret: true` の環境変数と `registryPassword` は API から値が返されないため、完全な比較ができません。これらが存在する場合は注意メッセージが表示されます。

### バージョンのアクティブ化 (activate)

```bash
# 最新バージョンをアクティブ化（デフォルト）
apprun-dedicated-application-provisioner activate -c apprun.yaml -a webapp

# 特定のバージョンをアクティブ化
apprun-dedicated-application-provisioner activate -c apprun.yaml -a webapp -t 2
```

| オプション | 説明 |
|-----------|------|
| `--config`, `-c` | 設定ファイルのパス（必須） |
| `--app`, `-a` | アプリケーション名（必須） |
| `--target`, `-t` | アクティブ化するバージョン（デフォルト: 最新バージョン） |

### 現在の設定をダンプ (dump)

```bash
apprun-dedicated-application-provisioner dump my-cluster
```

指定したクラスタの現在の設定を YAML 形式で出力します。既存環境の設定を取り込む際や、設定のバックアップに使用できます。

出力例:
```yaml
clusterName: my-cluster
cluster:
  servicePrincipalId: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
autoScalingGroups:
  - name: web-asg
    zone: is1a
    workerServiceClassPath: cloud/plan/ssd/1core-2gb
    minNodes: 2
    maxNodes: 10
    ...
loadBalancers:
  - name: web-lb
    autoScalingGroupName: web-asg
    serviceClassPath: cloud/plan/ssd/1core-2gb
    ...
applications:
  - name: webapp
    spec:
      cpu: 1000
      memory: 2048
      ...
```

**注意**:
- `letsEncryptEmail` は API から値を取得できないため、出力されません（設定の有無のみ確認可能）
- `registryPassword` や `secret: true` の環境変数の値は出力されません

## 設定ファイル

### 基本構造

```yaml
clusterName: "my-cluster"

# クラスタ設定（オプション）
cluster:
  letsEncryptEmail: "admin@example.com"
  servicePrincipalId: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

# AutoScalingGroup 設定（オプション）
autoScalingGroups:
  - name: "web-asg"
    zone: "is1a"
    workerServiceClassPath: "cloud/plan/ssd/1core-2gb"
    minNodes: 2
    maxNodes: 10
    nameServers:
      - "133.242.0.3"
      - "133.242.0.4"
    interfaces:
      - interfaceIndex: 0
        upstream: "shared"
        connectsToLB: true

# LoadBalancer 設定（オプション）
loadBalancers:
  - name: "web-lb"
    autoScalingGroupName: "web-asg"
    serviceClassPath: "cloud/plan/ssd/1core-2gb"
    nameServers:
      - "133.242.0.3"
    interfaces:
      - interfaceIndex: 0
        upstream: "shared"

# アプリケーション設定
applications:
  - name: "webapp"
    spec:
      cpu: 500           # mCPU (100-64000)
      memory: 1024       # MB (128-131072)
      scalingMode: "cpu" # "manual" or "cpu"
      minScale: 1
      maxScale: 3
      scaleInThreshold: 30
      scaleOutThreshold: 80
      image: "nginx:latest"  # 新規作成時のみ使用、既存アプリは継承
      exposedPorts:
        - targetPort: 80
          loadBalancerPort: 443
          useLetsEncrypt: true
          host:
            - "example.com"
          healthCheck:
            path: "/health"
            intervalSeconds: 30
            timeoutSeconds: 5
      env:
        - key: "APP_ENV"
          value: "production"
          secret: false
```

### 設定項目

#### クラスタ設定

| 項目 | 必須 | 説明 |
|------|------|------|
| `clusterName` | Yes | デプロイ先クラスタの名前 |
| `cluster` | No | クラスタ設定（既存クラスタの更新用） |
| `autoScalingGroups` | No | AutoScalingGroup 設定の配列 |
| `loadBalancers` | No | LoadBalancer 設定の配列 |
| `applications` | Yes | アプリケーション設定の配列 |

#### クラスタ設定 (cluster)

| 項目 | 必須 | 説明 |
|------|------|------|
| `letsEncryptEmail` | No | Let's Encrypt 用のメールアドレス |
| `servicePrincipalId` | Yes | サービスプリンシパル ID |

#### AutoScalingGroup 設定 (autoScalingGroups)

| 項目 | 必須 | 説明 |
|------|------|------|
| `name` | Yes | ASG 名（クラスタ内でユニーク） |
| `zone` | Yes | ゾーン（例: "is1a"） |
| `workerServiceClassPath` | Yes | ワーカーのサービスクラスパス |
| `minNodes` | Yes | 最小ノード数 |
| `maxNodes` | Yes | 最大ノード数 |
| `nameServers` | Yes | DNS サーバーのリスト |
| `interfaces` | Yes | ネットワークインターフェース設定 |

**注意**: ASG は更新をサポートしていません。設定を変更する場合は、削除して再作成されます。

#### LoadBalancer 設定 (loadBalancers)

| 項目 | 必須 | 説明 |
|------|------|------|
| `name` | Yes | LB 名（ASG 内でユニーク） |
| `autoScalingGroupName` | Yes | 所属する ASG の名前 |
| `serviceClassPath` | Yes | サービスクラスパス |
| `nameServers` | Yes | DNS サーバーのリスト |
| `interfaces` | Yes | ネットワークインターフェース設定 |

**注意**: LB は更新をサポートしていません。設定を変更する場合は、削除して再作成されます。LB は ASG に依存しているため、ASG を削除する前に LB が削除されます。

#### アプリケーション設定

| 項目 | 必須 | 説明 |
|------|------|------|
| `name` | Yes | アプリケーション名（クラスタ内でユニーク） |
| `spec` | Yes | アプリケーション仕様 |

#### アプリケーション仕様 (spec)

| 項目 | 必須 | 説明 | 継承 |
|------|------|------|------|
| `cpu` | No | CPU (mCPU) | Yes |
| `memory` | No | メモリ (MB) | Yes |
| `scalingMode` | No | `manual` または `cpu` | Yes |
| `fixedScale` | No | 固定スケール数（manual 時） | Yes |
| `minScale` | No | 最小スケール数（cpu 時） | Yes |
| `maxScale` | No | 最大スケール数（cpu 時） | Yes |
| `scaleInThreshold` | No | スケールイン閾値 (30-70) | Yes |
| `scaleOutThreshold` | No | スケールアウト閾値 (50-99) | Yes |
| `image` | No* | コンテナイメージ | Yes |
| `cmd` | No | 起動コマンド | Yes |
| `registryUsername` | No | レジストリユーザー名 | Yes |
| `registryPassword` | No | レジストリパスワード | Yes |
| `registryPasswordVersion` | No* | パスワードのバージョン番号 | No |
| `exposedPorts` | No | 公開ポート設定 | Yes |
| `env` | No | 環境変数 | Yes |

\* `image`: 新規アプリケーション作成時は必須
\* `registryPasswordVersion`: `registryPassword` 指定時は必須。パスワード変更時にバージョンを上げることで変更を検出

#### 公開ポート設定 (exposedPorts)

| 項目 | 必須 | 説明 |
|------|------|------|
| `targetPort` | Yes | アプリケーションのリッスンポート |
| `loadBalancerPort` | No | LB の公開ポート（null で LB なし） |
| `useLetsEncrypt` | Yes | Let's Encrypt の利用 |
| `host` | No | ホスト名ルーティング |
| `healthCheck` | No | ヘルスチェック設定 |

#### 環境変数設定 (env)

| 項目 | 必須 | 説明 |
|------|------|------|
| `key` | Yes | 環境変数名 |
| `value` | No | 値（secret 時は省略可） |
| `secret` | Yes | 秘密情報フラグ |
| `secretVersion` | No* | シークレットのバージョン番号 |

\* `secret: true` の場合は必須。値を変更する際にインクリメントすることで変更を検出

## 設定の継承ルール

既存のアプリケーションを更新する場合、YAML で指定していない項目は既存バージョンから自動的に継承されます。

- **image**: 常に既存バージョンから継承（新規アプリケーションの場合のみ YAML の値を使用）
- **その他の項目**: YAML で指定されていれば使用、省略されていれば既存を継承

これにより、CI/CD でのイメージデプロイと、このツールでの設定管理を分離できます。

## 状態ファイル

### 概要

このツールは、コンテナレジストリのパスワードなど、サーバーから取得できない情報の変更検出のために状態ファイルを使用します。

### ファイル仕様

- **ファイル名**: `<config名>.apprun-state.json`
  - 例: `apprun.yaml` の場合 → `apprun.apprun-state.json`
- **保存場所**: 設定ファイル（YAML）と同じディレクトリ
- **内容**: アプリケーションごとの `registryPasswordVersion` と `secretEnvVersions`

### ファイル構造

```json
{
  "version": 1,
  "applications": {
    "webapp": {
      "registryPasswordVersion": 1,
      "secretEnvVersions": {
        "DATABASE_URL": 1,
        "API_KEY": 2
      }
    }
  }
}
```

### 動作

1. **plan 時**: 状態ファイルのバージョンと YAML のバージョンを比較し、変更を検出
2. **apply 時**: 変更を適用後、状態ファイルを更新

### バージョン変更検出ロジック

`registryPasswordVersion` と `secretVersion`（env の secret=true 用）は同じロジックで変更を検出します：

| YAML のバージョン | 状態ファイルのバージョン | 判定 |
|-------------------|--------------------------|------|
| あり | なし | 変更あり（新規追加） |
| あり（一致） | あり | 変更なし |
| あり（不一致） | あり | 変更あり（更新） |
| なし | あり | 変更あり（削除） |
| なし | なし | 変更なし |

### 使用例

パスワードや secret 環境変数を変更する場合は、バージョンをインクリメントします：

```yaml
applications:
  - name: "webapp"
    spec:
      registryUsername: "myuser"
      registryPassword: "new-secret-password"
      registryPasswordVersion: 2  # 1 から 2 に変更
      env:
        - key: "DATABASE_URL"
          secret: true
          secretVersion: 2  # 1 から 2 に変更
```

### 注意事項

- 状態ファイルにはバージョン番号のみが保存されるため、Git で管理しても安全です
- 複数の設定ファイルを同じディレクトリで使用する場合、それぞれ独立した状態ファイルが作成されます
- `registryPassword` を指定する場合は `registryPasswordVersion` も必須です
- `secret: true` の環境変数には `secretVersion` が必須です

## 運用例

### 設定変更のみ（イメージはそのまま）

```yaml
applications:
  - name: "webapp"
    spec:
      cpu: 1000      # CPU を変更
      memory: 2048   # メモリを変更
      # その他は既存の設定を継承
```

### 新規アプリケーションの追加

```yaml
applications:
  - name: "new-service"
    spec:
      cpu: 500
      memory: 1024
      scalingMode: "manual"
      fixedScale: 2
      image: "myregistry/new-service:v1.0.0"
      exposedPorts:
        - targetPort: 8080
          loadBalancerPort: 443
          useLetsEncrypt: true
          host:
            - "new.example.com"
```

## ライセンス

MIT
