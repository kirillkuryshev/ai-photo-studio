import os
import urllib.request

import cv2
import numpy as np
import onnxruntime as ort

# Откуда пробуем скачать модель при первом запуске (порядок = приоритет).
MODEL_URLS = [
    "https://huggingface.co/Xenova/realesr-general-x4v3/resolve/main/onnx/model.onnx",
    "https://huggingface.co/Xenova/realesr-general-x4v3/resolve/main/onnx/model_fp16.onnx",
]


def download_model(model_path, urls=MODEL_URLS, progress_cb=None):
    """Скачивает ONNX-модель, если её ещё нет на диске.

    progress_cb(done_bytes, total_bytes) вызывается из этого же потока.
    Кидает RuntimeError с понятным описанием, если ничего не вышло.
    """
    if os.path.exists(model_path):
        return model_path

    model_dir = os.path.dirname(os.path.abspath(model_path))
    os.makedirs(model_dir, exist_ok=True)

    tmp_path = model_path + ".part"
    last_err = None
    for url in urls:
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
            with urllib.request.urlopen(req, timeout=60) as resp:
                total = int(resp.headers.get("Content-Length") or 0)
                done = 0
                with open(tmp_path, "wb") as f:
                    while True:
                        chunk = resp.read(1 << 16)
                        if not chunk:
                            break
                        f.write(chunk)
                        done += len(chunk)
                        if progress_cb:
                            progress_cb(done, total)
            # защита от html-страницы «404» вместо модели
            if done < 1_000_000:
                raise ValueError("скачался слишком маленький файл — это точно не модель")
            os.replace(tmp_path, model_path)
            return model_path
        except Exception as e:  # пробуем следующий источник
            last_err = e
            if os.path.exists(tmp_path):
                try:
                    os.remove(tmp_path)
                except OSError:
                    pass

    raise RuntimeError(
        "Не удалось скачать модель автоматически (последняя ошибка: "
        f"{last_err}).\n\n"
        "Скачайте файл realesr-general-x4v3.onnx вручную — например, поиском "
        "«realesr-general-x4v3 onnx» на huggingface.co — и положите его в папку:\n"
        f"{model_dir}"
    )


class RealESRUpscaler:
    """Апскейл x4 через ONNX-модель realesr-general-x4v3 (компактная GAN-сеть
    из Real-ESRGAN, ~5 МБ). Обрабатывает изображение тайлами с перекрытием,
    чтобы память оставалась низкой даже на слабых машинах.

    out_size в upscale() позволяет писать результат сразу в финальный размер
    (например, 4K): каждый тайл после нейросети уменьшается до своего места
    в финале — гигантский промежуточный x4-буфер не создаётся.
    """

    def __init__(self, model_path, tile=200, tile_pad=10, scale=4):
        if not os.path.isfile(model_path):
            raise FileNotFoundError(f"Файл модели не найден: {model_path}")

        self.scale = scale
        self.tile = tile
        self.tile_pad = tile_pad

        # если установлен onnxruntime-gpu / onnxruntime-directml —
        # ускорение подхватится автоматически, иначе CPU
        wanted = ("CUDAExecutionProvider", "DmlExecutionProvider", "CPUExecutionProvider")
        providers = [p for p in wanted if p in ort.get_available_providers()]
        try:
            self.session = ort.InferenceSession(model_path, providers=providers)
        except Exception:
            self.session = ort.InferenceSession(model_path, providers=["CPUExecutionProvider"])
        self.input_name = self.session.get_inputs()[0].name

        # прогрев: первый запуск onnxruntime оптимизирует граф — лучше сделать
        # это один раз на пустом тайле, чем тормозить на первом реальном
        self._run(np.zeros((tile, tile, 3), dtype=np.uint8))

    def _run(self, tile_bgr):
        rgb = tile_bgr[:, :, ::-1].astype(np.float32) / 255.0
        chw = np.transpose(rgb, (2, 0, 1))[None, ...]
        out = self.session.run(None, {self.input_name: chw})[0]
        out = np.clip(out[0], 0, 1)
        out = np.transpose(out, (1, 2, 0))
        return (out[:, :, ::-1] * 255.0).astype(np.uint8)

    def upscale(self, img_bgr, progress_cb=None, should_stop=None, out_size=None):
        """Возвращает BGR-изображение или None, если отменили через should_stop."""
        h, w = img_bgr.shape[:2]
        scale = self.scale
        nat_w, nat_h = w * scale, h * scale

        if out_size is None:
            out_w, out_h = nat_w, nat_h
        else:
            out_w = min(out_size[0], nat_w)
            out_h = min(out_size[1], nat_h)
        shrink = (out_w, out_h) != (nat_w, nat_h)
        kx = out_w / nat_w
        ky = out_h / nat_h

        result = np.empty((out_h, out_w, 3), dtype=np.uint8)

        tile, pad = self.tile, self.tile_pad
        n_tx = (w + tile - 1) // tile
        n_ty = (h + tile - 1) // tile
        total = n_tx * n_ty
        done = 0

        for ty in range(n_ty):
            for tx in range(n_tx):
                if should_stop is not None and should_stop():
                    return None

                x0, y0 = tx * tile, ty * tile
                x1, y1 = min(x0 + tile, w), min(y0 + tile, h)

                px0, py0 = max(x0 - pad, 0), max(y0 - pad, 0)
                px1, py1 = min(x1 + pad, w), min(y1 + pad, h)

                out_patch = self._run(img_bgr[py0:py1, px0:px1])

                cut_left = (x0 - px0) * scale
                cut_top = (y0 - py0) * scale
                useful = out_patch[cut_top:cut_top + (y1 - y0) * scale,
                                   cut_left:cut_left + (x1 - x0) * scale]

                if shrink:
                    # сразу кладём тайл в финальный (например, 4K) размер
                    dx0, dx1 = round(x0 * scale * kx), round(x1 * scale * kx)
                    dy0, dy1 = round(y0 * scale * ky), round(y1 * scale * ky)
                    useful = cv2.resize(useful, (dx1 - dx0, dy1 - dy0),
                                        interpolation=cv2.INTER_AREA)
                    result[dy0:dy1, dx0:dx1] = useful
                else:
                    result[y0 * scale:y1 * scale, x0 * scale:x1 * scale] = useful

                done += 1
                if progress_cb:
                    progress_cb(done, total)

        return result


class FourKUpscaler(RealESRUpscaler):
    """Апскейл до 4K (3840x2160) с использованием ONNX-модели.
    Принудительно ограничивает максимальный размер вывода до 4K,
    сохраняя пропорции изображения.
    """

    def __init__(self, model_path, tile=200, tile_pad=10, scale=4):
        super().__init__(model_path, tile, tile_pad, scale)

    def upscale(self, img_bgr, progress_cb=None, should_stop=None, out_size=None):
        h, w = img_bgr.shape[:2]
        
        # Если out_size не задан, рассчитываем его так, чтобы максимальная сторона была 3840 (4K)
        if out_size is None:
            max_side = 3840
            if max(h, w) * self.scale > max_side:
                ratio = max_side / float(max(h, w) * self.scale)
                out_size = (int(w * self.scale * ratio), int(h * self.scale * ratio))
        
        return super().upscale(img_bgr, progress_cb, should_stop, out_size)