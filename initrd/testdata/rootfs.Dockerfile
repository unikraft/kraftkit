FROM debian:trixie-20261104@sha256:9c6e381421d0f48d1b8133e573f33c185ee16cbf505eda9c4a8a4791b25ea8a6

# Create a directory
RUN mkdir -p /out/a/b/c

# Set the working directory
WORKDIR /out/a/b/c

# Create a file
RUN echo "Hello, World" > ./d

# Create a symbolic linked file
RUN ln -s ./d ./e-symlink

# Create a hard linked file
RUN ln ./d ./f-hardlink

# Create a recursive symolic link
RUN ln -s . ./g-recursive-symlink

# Create a blank file system
FROM scratch

# Copy the directory from the previous stage
COPY --from=0 /out /
