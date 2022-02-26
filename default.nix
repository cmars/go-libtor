{ pkgs ? import <nixpkgs> {} }:
  pkgs.mkShell {
    nativeBuildInputs = with pkgs.buildPackages; [
      # Local development for NixOS
      go_1_17 tor openssl_1_1 libevent zlib

      # Linux CI environment uses Podman for local execution of containers.
      podman

      # macOS amd64 CI environment uses KVM.
      python39 qemu_full libvirt virt-manager

      # TODO: macOS m1 env
      # TODO: Windows MSYS
    ];
    shellHook = ''
      export GOPATH="$HOME/.cache/gopaths/$(sha256sum <<<$(pwd) | awk '{print $1}')"
    '';
}

