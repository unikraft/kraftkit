{
  description = "Build and use highly customized and ultra-lightweight unikernel VMs. ";

  # Nixpkgs / NixOS version to use.
  inputs.nixpkgs.url = "nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      # to work with older version of flakes
      lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";
      # Generate a user-friendly version number.
      version = builtins.substring 0 8 lastModifiedDate;

      supportedSystems = [ "x86_64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          kraft = pkgs.buildGo118Module {
            pname = "kraft";
            inherit version;
            src = ./.;

            buildInputs = [ pkgs.libgit2_1_3_0 ];
            nativeBuildInputs = [ pkgs.pkg-config ];

            subPackages = [ "cmd/kraft" ];
            #vendorSha256 = pkgs.lib.fakeSha256;
            vendorSha256 = "sha256-cQkX3333pkRkXk2Y43buUvBosXtBdfig+kvMRwD0Iew=";
          };
        });

      defaultPackage = forAllSystems (system: self.packages.${system}.kraft);
    };
}
