# coding: utf-8
class Kss < Formula
  desc "Enhanced Kubernetes Pod Inspection"
  homepage "https://github.com/chmouel/kss"
  version "0.4.0"

  depends_on "go" => :build
  depends_on "fzf"
  depends_on "kubectl"

  url "https://github.com/chmouel/kss/tarball/#{version}"
  sha256 "08453cac989ad58c28eb4e3ba3cb68b9ebfeec4304cda26ac76ae4efc01bb088"

  def install
    system "go", "build", "-o", "kss", "./cmd/kss"
    bin.install "kss"
    bin.install_symlink "kss" => "kubectl-kss"

    # Generate completions
    system "./kss --completion bash > kss.bash"
    system "./kss --completion zsh > _kss"
    bash_completion.install "kss.bash" => "kss"
    zsh_completion.install "_kss"

    prefix.install_metafiles
  end

end
