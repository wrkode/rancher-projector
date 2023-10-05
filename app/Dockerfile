FROM registry.suse.com/bci/golang:1.21
ARG USERNAME=projector
ARG USER_UID=1000
ARG USER_GID=$USER_UID
RUN groupadd --gid $USER_GID $USERNAME && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o projector .
RUN chown $USER_GID:$USERNAME $USERNAME
USER $USERNAME
CMD ["./projector"]