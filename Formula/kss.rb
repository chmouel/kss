# coding: utf-8
class Kss < Formula
  desc "Kubernetes pod status on steroid 💉"
  homepage "https://github.com/chmouel/kss"
  version "0.2.1"

  depends_on "fzf"
  depends_on "kubectl"

  url "https://github.com/chmouel/kss/tarball/#{version}"
  sha256 "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

  def install
    bin.install "kss" => "kss"
    bin.install_symlink "kss" => "kubectl-kss"
    zsh_completion.install "_kss"
    prefix.install_metafiles
  end

end
