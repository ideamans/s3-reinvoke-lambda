# s3-reinvoke-lambda

## S3オブジェクト作成イベントによるLambda関数の再実行

S3バケットでオブジェクトが作成(または更新)されたときに、Lambda関数を起動して何らかの処理を行うユースケースがあります。

Lambda関数を設定した後のオブジェクトについては、イベント発生に基づきLambda関数が実行されますが、それ以前に作成されたオブジェクトについては当然ながらLambda関数は実行されません。

その用途のため、[Amazon S3 バッチオペレーション](https://aws.amazon.com/jp/s3/features/batch-operations/)がありますが、利用には少々手間がかかります。

そこで既存のS3オブジェクトに対し、オブジェクト作成イベントに基づくLambda関数を簡単に実行するために開発したのが`s3-reinvoke-lambda`コマンドです。

## クロスプラットフォーム

`s3-reinvoke-lambda`コマンドはGo言語による複数のプラットフォームに対応したCLIコマンドです。

次のOS向けのビルド済みファイルを提供していますが、必要に応じてコンパイルして利用ください。

- Linux
- Windows
- MacOS
- FreeBSD

## 使い方

Linuxでの利用を例に解説します。

### AWSの設定

AWS CLIと認証を設定済みの場合は次のステップに進んでください。

```bash
aws configure
```

該当S3バケットに対する`s3:ListObjectV2`、Lambda関数に対する`InvokeFunction`が許可されたユーザーを利用ください。

### s3-reinvoke-lambdaコマンドのインストール

プログラムをダウンロードし、実行可能な状態にしてください。

```bash
wget -o s3-reinvoke-lambda [ダウンロードURL]
sudo mv s3-reinvoke-lambda /usr/local/bin
sudo chmod +x /usr/local/bin/s3-reinvoke-lambda
```

### s3-reinvoke-lambdaコマンドの実行

対象のバケットを`the-bucket`、Lambda関数名を`the-function`とします。

次のコマンドを実行すると、`the-bucket`に対する全オブジェクトについて、オブジェクト作成イベントをパラメータとして`the-function`を起動します。

```bash
s3-reinvoke-lambda the-bucket the-function
```

プロファイルの選択やリージョンの指定はAWS CLIコマンドと同様に可能です。

```bash
AWS_PROFILE=my-profile AWS_REGION=ap-northeast-1 s3-reinvoke-lambda the-bucket the-function
```

## オプション

次のようにオプションを指定できます。

```bash
s3-reinvoke-lambda -P 50 the-bucket the-function
```

### 並列実行数

- `-P N` `--parallel N` Lambda関数を起動する並列数です。デフォルトは100です。

### 対象オブジェクトの絞り込み

- `-p プレフィックス` `--prefix` 指定のプレフィックスを持つキー名を処理の対象とします(例 `images/`)
- `-b 時刻` `--mod-before` 指定の時刻(RFC3339で指定)より前のオブジェクトのみ対象とします(例 `2024-06-21T19:49:00+09:00`)。
- `-x 拡張子リスト` `--extensions 拡張子リスト` カンマ区切りの小文字による拡張子リスト(例 .jpg,.jpeg,.png)を対象とします。キー名の拡張子は大文字小文字を区別しません。

実際の状況として、Lambda関数を設定した以後は自動的に実行されます。`-b`オプションにその時刻を指定して、対象をそれ以前のオブジェクトに絞るのは有効な方法です。

### 処理の再開

- `-a キー名` `--start-after` 指定のキー名より後のオブジェクトから開始します。

なお、ログに表示されるオブジェクトのキー名は、あくまでLambda関数の実行完了順であり、S3における配置の順序とは異なります。

ログの最終行のキー名を指定するのではなく、キー名を昇順ソートした最後のキー名を指定してください。
