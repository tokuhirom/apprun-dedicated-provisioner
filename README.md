# apprun-dedicated-application-provisioner

さくらのクラウド AppRun 専有型のアプリケーション設定を YAML ファイルで管理し、同期するツールです。

## 特徴

- **Infrastructure as Code**: YAML ファイルでアプリケーション設定を宣言的に管理
- **plan/apply**: Terraform 風の2段階操作で安全に変更を適用
- **設定の継承**: YAML で指定していない項目は既存バージョンの設定を自動的に引き継ぎ
- **image の分離**: コンテナイメージは既存バージョンから継承（CI/CD でのデプロイと設定管理を分離）

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

## 設定ファイル

### 基本構造

```yaml
clusterName: "my-cluster"

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
| `applications` | Yes | アプリケーション設定の配列 |

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
| `exposedPorts` | No | 公開ポート設定 | Yes |
| `env` | No | 環境変数 | Yes |

\* 新規アプリケーション作成時は必須

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

## 設定の継承ルール

既存のアプリケーションを更新する場合、YAML で指定していない項目は既存バージョンから自動的に継承されます。

- **image**: 常に既存バージョンから継承（新規アプリケーションの場合のみ YAML の値を使用）
- **その他の項目**: YAML で指定されていれば使用、省略されていれば既存を継承

これにより、CI/CD でのイメージデプロイと、このツールでの設定管理を分離できます。

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
