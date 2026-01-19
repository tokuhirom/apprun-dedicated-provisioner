{
  description = "CLI tool for managing Sakura Cloud AppRun Dedicated application provisioning";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs = { self, nixpkgs, flake-utils, gomod2nix }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        gomod2nixPkgs = gomod2nix.legacyPackages.${system};
        version = if self ? shortRev then self.shortRev else "dev";
      in
      {
        packages = {
          default = self.packages.${system}.apprun-dedicated-provisioner;

          apprun-dedicated-provisioner = gomod2nixPkgs.buildGoApplication {
            pname = "apprun-dedicated-provisioner";
            inherit version;

            src = ./.;
            modules = ./gomod2nix.toml;

            subPackages = [ "cmd/apprun-dedicated-provisioner" ];

            meta = with pkgs.lib; {
              description = "CLI tool for managing Sakura Cloud AppRun Dedicated application provisioning";
              homepage = "https://github.com/tokuhirom/apprun-dedicated-provisioner";
              license = licenses.mit;
              maintainers = [ ];
              mainProgram = "apprun-dedicated-provisioner";
            };
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            gomod2nix.packages.${system}.default
          ];
        };
      }
    );
}
