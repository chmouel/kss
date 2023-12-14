# coding: utf-8
class Kss < Formula
  desc "Kubernetes pod status on steroid 💉"
  homepage "https://github.com/chmouel/kss"
  version "0.4.0"

  depends_on "fzf"
  depends_on "kubectl"

  url "https://github.com/chmouel/kss/tarball/#{version}"
  sha256 "08453cac989ad58c28eb4e3ba3cb68b9ebfeec4304cda26ac76ae4efc01bb088"

  def install
    bin.install "kss" => "kss"
    bin.install_symlink "kss" => "kubectl-kss"
    zsh_completion.install "_kss"
    prefix.install_metafiles
  end

end
