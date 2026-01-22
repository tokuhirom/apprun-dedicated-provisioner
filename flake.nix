{
  description = "CLI tool for managing Sakura Cloud AppRun Dedicated application provisioning";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = "0.0.32";

      # SHA256 hashes from goreleaser checksums.txt
      hashes = {
        "x86_64-linux" = "8abd6d5f93cdfcade18420fe9db000efa8e0d25e9f2231d4658a8ae83c80c3bc";
        "aarch64-linux" = "cf4d6925a38e8334d301f2d92853fe9be714dddf5c43d03ba8214af17378b1d1";
        "x86_64-darwin" = "e74a981b2f0a54c79f53f6011a61cd74fcc982c2e39d446acb978ead344d3f09";
        "aarch64-darwin" = "19b79da681456a0f69a8659f2c05f9c47f330754a836f1cfdedb825968d33ce1";
      };

      # Map Nix system to goreleaser naming (unquoted keys to avoid sed replacement)
      goreleaserNames = {
        x86_64-linux = "linux_amd64";
        aarch64-linux = "linux_arm64";
        x86_64-darwin = "darwin_amd64";
        aarch64-darwin = "darwin_arm64";
      };

    in
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        goreleaserSystem = goreleaserNames.${system};
      in
      {
        packages = {
          default = self.packages.${system}.apprun-dedicated-provisioner;

          apprun-dedicated-provisioner = pkgs.stdenv.mkDerivation {
            pname = "apprun-dedicated-provisioner";
            inherit version;

            src = pkgs.fetchurl {
              url = "https://github.com/tokuhirom/apprun-dedicated-provisioner/releases/download/v${version}/apprun-dedicated-provisioner_${version}_${goreleaserSystem}.tar.gz";
              sha256 = hashes.${system};
            };

            sourceRoot = ".";

            installPhase = ''
              mkdir -p $out/bin
              cp apprun-dedicated-provisioner $out/bin/
              chmod +x $out/bin/apprun-dedicated-provisioner
            '';

            meta = with pkgs.lib; {
              description = "CLI tool for managing Sakura Cloud AppRun Dedicated application provisioning";
              homepage = "https://github.com/tokuhirom/apprun-dedicated-provisioner";
              license = licenses.mit;
              maintainers = [ ];
              mainProgram = "apprun-dedicated-provisioner";
              platforms = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
            };
          };
        };

        # Development shell (still builds from source for development)
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
          ];
        };
      }
    );
}
