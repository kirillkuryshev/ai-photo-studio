# Photo Studio AI

Photo Studio AI is my pet project for experimenting with AI-powered photo enhancement and full-stack development. Users can create an account, upload an image, view the processed result, and browse their processing history.

## Features

- **Restore Old Photos** enhances and upscales old photos with Real-ESRGAN. The current implementation does not specifically remove scratches or reconstruct missing parts of an image.
- **Upscale Image to 4K** increases resolution while preserving the aspect ratio and limiting the longest side to 3840 pixels.
- **Colorize Photos** adds color to black-and-white images using a neural network with a ResNet50 encoder.
- Account registration, token-based authentication, and a per-user history with original and processed images.

The interface also displays **Enhance Quality**, **Remove Noise**, and **Remove Scratches** as unavailable features.

## Project structure

| Directory | Purpose |
| --- | --- |
| `frontend/` | React, TypeScript, and Vite user interface |
| `photo-studio-api/` | Laravel API, Sanctum authentication, SQLite database, image storage, and history |
| `photo-studio-ai-service/` | FastAPI image processing service using ONNX Runtime and PyTorch |
| `photo-studio-go/` | Separate Go implementation of the HTTP API; it is not needed for the setup below |

The main request flow is **browser → Laravel API (`:8000`) → Python AI service (`:8001`)**. Laravel stores history in SQLite and images in its public storage directory. The Go server also uses port `8000`, so it cannot run alongside Laravel on the same port. Its database and token handling are not currently interchangeable with Laravel Sanctum.

## Local setup

The commands below use PowerShell. Install PHP **8.3+** with the extensions required by Laravel and SQLite, Composer, Node.js with npm, and Python with pip. Go is only needed if you want to work with the separate Go server.

### 1. Python AI service

From the repository root:

```powershell
cd photo-studio-ai-service
py -m venv venv
.\venv\Scripts\python -m pip install -r requirements.txt
.\venv\Scripts\python -m pip install torch torchvision scikit-image
```

The service imports `torch`, `torchvision`, and `scikit-image`, but these packages are currently missing from `requirements.txt`, so install them separately. The appropriate PyTorch installation may depend on your CPU or GPU.

Colorization requires `photo-studio-ai-service/weights/colorizer_v10.pt`. This file is not included in the Git repository and is not downloaded automatically. **The AI service will not start without it**, even if you only want to use the upscaling features, because the colorizer is initialized at startup. The weights must be compatible with `ColorizerNet` in `colorize.py`.

On first startup, the service attempts to download the ONNX model to `photo-studio-ai-service/models_onnx/realesr-general-x4v3.onnx`. You can also place a compatible model there manually. Model and weight files are excluded from Git.

After the models are available, start the service from `photo-studio-ai-service`:

```powershell
.\venv\Scripts\python main.py
```

It listens at `http://127.0.0.1:8001`.

### 2. Laravel API

In another terminal, from the repository root:

```powershell
cd photo-studio-api
composer install
Copy-Item .env.example .env
New-Item database/database.sqlite -ItemType File -Force
php artisan key:generate
php artisan migrate
php artisan storage:link
php artisan serve --host=127.0.0.1 --port=8000
```

The example environment uses SQLite. Laravel calls the AI service at `http://127.0.0.1:8001`. The `storage:link` command makes images available in the history view.

### 3. Frontend

In a third terminal, from the repository root:

```powershell
cd frontend
npm ci
npm run dev
```

Open the URL printed by Vite, usually `http://localhost:5173`. The frontend currently hardcodes the API address as `localhost` or `127.0.0.1` on port `8000`; update those URLs in the source code if you run the API on another host.

## API

Public endpoints: `POST /api/signup` and `POST /api/login`. After logging in, send the returned token in the `Authorization: Bearer <token>` header.

Authenticated endpoints:

| Method and path | Action |
| --- | --- |
| `POST /api/photos/transform` | Enhance and upscale an old photo |
| `POST /api/photos/upscale-4k` | Upscale an image with a 4K size limit |
| `POST /api/photos/colorize` | Colorize an image |
| `GET /api/user-history` | Get the current user's history |
| `POST /api/logout` | Log out |

For image processing requests, send the image as `multipart/form-data` in a field named `photo`. The API returns the processed image. The frontend loads history with a separate request.

## Current limitations

- This is a pet project configured for local development; deployment settings are not included.
- Colorizer weights must be supplied separately. A fresh clone does not contain everything needed to start the AI service.
- The frontend displays the processed image but does not yet have a download button or robust request-error handling. Images are also saved in the user's history.
- The Go server is an alternative API implementation and is not used in the setup above.
- The existing automated tests do not cover the image-processing workflow.
