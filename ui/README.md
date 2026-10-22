# ALS UI

The frontend uses Vue 3 and Vite. Use Node.js 24.22.1 (see [.nvmrc](.nvmrc)) and npm.

From the repository root, initialize the LibreSpeed submodule before building:

```sh
git submodule update --init --recursive
```

Run the following commands from `ui/`:

```sh
npm ci
npm run dev
```

The development server proxies `/session` and `/method` to the backend at `http://127.0.0.1:8080`.

To check and build the frontend:

```sh
npm run lint
npm test
npm run build
```

Use `npm run lint:fix` for automatic lint fixes or `npm run format` to format source files. Commit `package-lock.json` whenever dependencies change so local, Docker, and GitHub Actions builds install the same versions.

To embed the frontend in a local backend build, run these commands from the repository root with Go 1.27.3:

```sh
cp -R ui/dist/. backend/embed/ui/
go -C backend build -o ../tmp/als .
```
