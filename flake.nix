{
  description = "WINGS V DeX desktop application";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
      lib = pkgs.lib;
      src = ./.;
      goBuilder = pkgs.buildGoModule.override { go = pkgs.go_1_27; };
      goDepsHash = "sha256-hXKMRxQJKXXUMOESe9J6Gr/78HyOEUuOrubbUbr0m4U=";
      goModules = (goBuilder {
        pname = "wingsv-dex-modules";
        version = "0.0.0-dev";
        inherit src;
        vendorHash = goDepsHash;
        proxyVendor = true;
      }).goModules;
      helpers = import ./nix/helpers.nix { inherit pkgs src; };
      frontend = import ./nix/frontend.nix { inherit pkgs src goModules; };
      app = goBuilder {
        pname = "wingsv-dex";
        version = "0.0.0-dev";
        inherit src;
        vendorHash = goDepsHash;
        proxyVendor = true;
        nativeBuildInputs = [ pkgs.pkg-config ];
        buildInputs = with pkgs; [ gtk4 webkitgtk_6_0 libsoup_3 ];
        env.CGO_ENABLED = "1";
        tags = [ "production" ];
        ldflags = [
          "-s" "-w"
          "-X github.com/WINGS-N/wingsv-dex/internal/services.AppVersion=0.0.0-dev"
          "-X github.com/WINGS-N/wingsv-dex/internal/wg.nixNftPath=${lib.getExe pkgs.nftables}"
        ];
        postPatch = ''
          mkdir -p frontend/dist
          cp -r ${frontend}/dist/. frontend/dist/
        '';
        subPackages = [ "." ];
        doCheck = false;
      };
      package = pkgs.stdenvNoCC.mkDerivation {
        pname = "wingsv-dex";
        version = "0.0.0-dev";
        dontUnpack = true;
        nativeBuildInputs = [ pkgs.makeWrapper ];
        installPhase = ''
          mkdir -p $out/bin $out/share/applications $out/share/icons/hicolor/256x256/apps
          install -m755 ${app}/bin/wingsv-dex $out/bin/wingsv-dex
          install -m755 ${helpers.vkturn}/bin/vkturn $out/bin/vkturn
          install -m755 ${helpers.xray}/bin/xray $out/bin/xray
          install -m755 ${helpers.byedpi}/bin/byedpi $out/bin/byedpi
          install -m644 ${src}/build/linux/wingsv-dex.desktop $out/share/applications/wingsv-dex.desktop
          install -m644 ${src}/build/appicon.png $out/share/icons/hicolor/256x256/apps/wingsv-dex.png
          wrapProgram $out/bin/wingsv-dex \
            --prefix PATH : /run/wrappers/bin:${lib.makeBinPath [ pkgs.coreutils pkgs.nftables ]} \
            --prefix XDG_DATA_DIRS : ${lib.makeSearchPath "share" [ pkgs.gsettings-desktop-schemas pkgs.gtk4 pkgs.webkitgtk_6_0 ]} \
            --prefix GIO_EXTRA_MODULES : ${lib.makeSearchPath "lib/gio/modules" [ pkgs.glib-networking ]}
        '';
      };
    in {
      packages.${system}.default = package;
      nixosModules.default = import ./nix/module.nix { inherit package; };
      apps.${system}.default = { type = "app"; program = "${package}/bin/wingsv-dex"; };

      devShells.${system}.default = pkgs.mkShell {
        packages = with pkgs; [
          go_1_27 nodejs pnpm go-task pkg-config gcc gnumake git nftables
          protobuf protoc-gen-go protoc-gen-go-grpc gtk4 webkitgtk_6_0 libsoup_3
        ];
        shellHook = ''
          export CGO_ENABLED=1
          export GOFLAGS="-buildvcs=false"
        '';
      };

      checks.${system}.nethelper = pkgs.testers.runNixOSTest (import ./nix/tests/nethelper.nix {
        inherit nixpkgs;
        inherit package;
        module = self.nixosModules.default;
      });
    };
}
