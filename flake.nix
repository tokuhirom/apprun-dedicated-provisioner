{
  description = "CLI tool for managing Sakura Cloud AppRun Dedicated application provisioning";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = "0.0.30";

      # SHA256 hashes from goreleaser checksums.txt
      hashes = {
        "x86_64-linux" = "34a8d260ea25a4bcf9b9911c7f8869eb19ebf07e82fed545d63dcb9235b9fff1";
        "aarch64-linux" = "1cbfd2edbd95b3824f33a78b9a1a6f607fd76937bc146c765f85b7261754c5d2";
        "x86_64-darwin" = "a73e257c6a34492d87805dd1c80797b9d48834ce12993762fabe211c2dfa29c5";
        "aarch64-darwin" = "c34e0f34e961c149fda31e1be6ce35f0705d60f5bcf8d80b6a39cb0a325192ea";
      };

      # Map Nix system to goreleaser naming
      systemToGoreleaser = system: {
        "x86_64-linux" = "34a8d260ea25a4bcf9b9911c7f8869eb19ebf07e82fed545d63dcb9235b9fff1";
        "aarch64-linux" = "1cbfd2edbd95b3824f33a78b9a1a6f607fd76937bc146c765f85b7261754c5d2";
        "x86_64-darwin" = "a73e257c6a34492d87805dd1c80797b9d48834ce12993762fabe211c2dfa29c5";
        "aarch64-darwin" = "c34e0f34e961c149fda31e1be6ce35f0705d60f5bcf8d80b6a39cb0a325192ea";
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
