{
  description = "WINGS V DeX development and local runner";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
      runner = pkgs.writeShellScriptBin "wingsv-dex" ''
        exec ${pkgs.nix}/bin/nix develop . -c ${pkgs.bash}/bin/bash ${./scripts/nix-run.sh} "$@"
      '';
    in {
      packages.${system}.default = runner;
      apps.${system}.default = {
        type = "app";
        program = "${runner}/bin/wingsv-dex";
      };

      devShells.${system}.default = pkgs.mkShell {
        packages = with pkgs; [
          go_1_27
          nodejs
          pnpm
          go-task
          pkg-config
          gcc
          gnumake
          git
          nftables
          protobuf
          protoc-gen-go
          protoc-gen-go-grpc
          gtk4
          webkitgtk_6_0
          libsoup_3
        ];

        shellHook = ''
          export CGO_ENABLED=1
          export GOFLAGS="-buildvcs=false"
        '';
      };
    };
}
