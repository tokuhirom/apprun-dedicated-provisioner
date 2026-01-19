{
  description = "CLI tool for managing Sakura Cloud AppRun Dedicated application provisioning";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = "0.0.31";

      # SHA256 hashes from goreleaser checksums.txt
      hashes = {
        "x86_64-linux" = "dd91ec1afa07d6b52b9825b935afb77b75d64948196a49539c07a4cf024f539c";
        "aarch64-linux" = "73ee6d041261155c85a479688c4e626ca8ac8a09b32e5725a497618aa5d836a7";
        "x86_64-darwin" = "10092e7454ee2e4cee57f9cb08a6bfb03d4097446fe473bcbca4910b18b0a628";
        "aarch64-darwin" = "f0b77c6992a9b9cbc55794dfbbe624cd7ada73265aff99d9f0a54d8794714902";
      };

      # Map Nix system to goreleaser naming
      systemToGoreleaser = system: {
        "x86_64-linux" = "dd91ec1afa07d6b52b9825b935afb77b75d64948196a49539c07a4cf024f539c";
        "aarch64-linux" = "73ee6d041261155c85a479688c4e626ca8ac8a09b32e5725a497618aa5d836a7";
        "x86_64-darwin" = "10092e7454ee2e4cee57f9cb08a6bfb03d4097446fe473bcbca4910b18b0a628";
        "aarch64-darwin" = "f0b77c6992a9b9cbc55794dfbbe624cd7ada73265aff99d9f0a54d8794714902";
      }.${system};

    in
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        goreleaserSystem = systemToGoreleaser system;
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
