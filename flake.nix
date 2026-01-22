{
  description = "CLI tool for managing Sakura Cloud AppRun Dedicated application provisioning";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = "0.0.33";

      # SHA256 hashes from goreleaser checksums.txt
      hashes = {
        "x86_64-linux" = "54d573b2fe46c93ce48cb8aa006251c5fbae2bf58d38f351ad505cc314e57730";
        "aarch64-linux" = "8527ffd0c71781ef1ed91015717107ec496a58edb4d46fd93562781581202e8a";
        "x86_64-darwin" = "86c44c70b9adb508efbed9e94960b0e1a54f86446bab1e386ee5d73811618735";
        "aarch64-darwin" = "83d58430cce4df2ff409c19cd88ba78e385e624963212d147498a91f91aa1d86";
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
