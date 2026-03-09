class Cllmhub < Formula
  desc "Turn your local LLM into a production API"
  homepage "https://github.com/cllmhub/cllmhub-cli"
  version "0.4.10"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-darwin-arm64"
      sha256 "fc017da9aa6d6eda89514fe7828b07e9625008f5226b380436b7c8de29725567"
    else
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-darwin-amd64"
      sha256 "fc017da9aa6d6eda89514fe7828b07e9625008f5226b380436b7c8de29725567"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-linux-arm64"
      sha256 "fc017da9aa6d6eda89514fe7828b07e9625008f5226b380436b7c8de29725567"
    else
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-linux-amd64"
      sha256 "fc017da9aa6d6eda89514fe7828b07e9625008f5226b380436b7c8de29725567"
    end
  end

  def install
    binary = Dir.glob("cllmhub-*").first || "cllmhub"
    bin.install binary => "cllmhub"
  end

  test do
    assert_match "cllmhub", shell_output("#{bin}/cllmhub --version")
  end
end
