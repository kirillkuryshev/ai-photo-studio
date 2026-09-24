import os
import cv2
import numpy as np
from PIL import Image, ImageOps
import io
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import Response
import uvicorn

from upscaler import RealESRUpscaler, FourKUpscaler, download_model

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
MODEL_PATH = os.path.join(BASE_DIR, "models_onnx", "realesr-general-x4v3.onnx")

app = FastAPI()
upscaler = None
upscaler_4k = None

@app.on_event("startup")
def startup_event():
    global upscaler, upscaler_4k
    if not os.path.exists(MODEL_PATH):
        download_model(MODEL_PATH)
    upscaler = RealESRUpscaler(MODEL_PATH, scale=4)
    upscaler_4k = FourKUpscaler(MODEL_PATH, scale=4)

@app.post("/restore")
async def enhance_image(request: Request):
    try:
        content_type = request.headers.get("content-type", "")
        contents = None
        
        # Если Laravel прислал форму (multipart/form-data)
        if "multipart/form-data" in content_type:
            form = await request.form()
            for _, field in form.items():
                if hasattr(field, "file"):  # Ищем любой прикрепленный файл
                    contents = await field.read()
                    break
            if not contents:
                raise Exception("Файл не найден внутри формы")
        else:
            # Если Laravel прислал голые байты
            contents = await request.body()

        # Открываем байты как картинку
        with Image.open(io.BytesIO(contents)) as pil_img:
            pil_img = ImageOps.exif_transpose(pil_img)
            rgb = pil_img.convert("RGB")
        img = cv2.cvtColor(np.array(rgb), cv2.COLOR_RGB2BGR)
        
    except Exception as e:
        # Если вылетит ошибка, мы точно увидим её причину в терминале
        print(f"Ошибка парсинга: {str(e)}")
        raise HTTPException(status_code=400, detail=f"Ошибка чтения файла: {str(e)}")

    # Прогоняем через нейросеть
    result = upscaler.upscale(img)

    # Кодируем обратно в PNG
    is_success, buffer = cv2.imencode(".png", result)
    if not is_success:
        raise HTTPException(status_code=500, detail="Ошибка кодирования результата")

    # Возвращаем готовую картинку
    return Response(content=buffer.tobytes(), media_type="image/png")


@app.post("/upscale-4k")
async def upscale_4k_image(request: Request):
    try:
        content_type = request.headers.get("content-type", "")
        contents = None
        
        if "multipart/form-data" in content_type:
            form = await request.form()
            for _, field in form.items():
                if hasattr(field, "file"):
                    contents = await field.read()
                    break
            if not contents:
                raise Exception("Файл не найден внутри формы")
        else:
            contents = await request.body()

        with Image.open(io.BytesIO(contents)) as pil_img:
            pil_img = ImageOps.exif_transpose(pil_img)
            rgb = pil_img.convert("RGB")
        img = cv2.cvtColor(np.array(rgb), cv2.COLOR_RGB2BGR)
        
    except Exception as e:
        print(f"Ошибка парсинга: {str(e)}")
        raise HTTPException(status_code=400, detail=f"Ошибка чтения файла: {str(e)}")

    # Прогоняем через 4K нейросеть
    result = upscaler_4k.upscale(img)

    is_success, buffer = cv2.imencode(".png", result)
    if not is_success:
        raise HTTPException(status_code=500, detail="Ошибка кодирования результата")

    return Response(content=buffer.tobytes(), media_type="image/png")

if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8001)