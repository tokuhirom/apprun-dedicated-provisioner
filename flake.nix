{
  description = "CLI tool for managing Sakura Cloud AppRun Dedicated application provisioning";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = "0.0.35";

      # SHA256 hashes from goreleaser checksums.txt
      hashes = {
        "x86_64-linux" = "0ea50ba94df1e4de81ed10fd620ddbbfe6e8bd5c7f498a20602d007d7eb66000";
        "aarch64-linux" = "47bb79dd27c6e348f3eada3088c66c02ef7f06331344c8bc1e3bc25c48f9bc02";
        "x86_64-darwin" = "738def6ad0ac5171c60e8108aa83a5335ac421aa76745f70290c84ec3d634de1";
        "aarch64-darwin" = "f7ed6fb63dfa49613c8da85d8afe1182d2917ce9e3dfa406b377e321a58f9d3f";
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
