import io
import os

import numpy as np
import torch
import torch.nn as nn
import torchvision.models as models
from PIL import Image
from skimage.color import rgb2lab, lab2rgb


# =========================================================
# МОДЕЛЬ
# =========================================================

class DecoderBlock(nn.Module):
    def __init__(self, in_ch, out_ch):
        super().__init__()

        self.up = nn.Upsample(
            scale_factor=2,
            mode="bilinear",
            align_corners=True,
        )

        self.conv1 = nn.Conv2d(
            in_ch,
            out_ch,
            3,
            padding=1,
        )

        self.conv2 = nn.Conv2d(
            out_ch,
            out_ch,
            3,
            padding=1,
        )

        self.bn1 = nn.BatchNorm2d(out_ch)
        self.bn2 = nn.BatchNorm2d(out_ch)

        self.act = nn.ReLU(inplace=True)

    def forward(self, x, skip=None):
        x = self.up(x)

        if skip is not None:
            x = torch.cat(
                [
                    x,
                    skip[:, :, :x.shape[2], :x.shape[3]],
                ],
                dim=1,
            )

        x = self.act(
            self.bn1(
                self.conv1(x)
            )
        )

        x = self.act(
            self.bn2(
                self.conv2(x)
            )
        )

        return x


class ColorizerNet(nn.Module):
    def __init__(self, pretrained=False):
        super().__init__()

        resnet = models.resnet50(
            weights=(
                models.ResNet50_Weights.DEFAULT
                if pretrained
                else None
            )
        )

        self.conv1 = resnet.conv1
        self.bn1 = resnet.bn1
        self.relu = resnet.relu
        self.maxpool = resnet.maxpool

        self.layer1 = resnet.layer1
        self.layer2 = resnet.layer2
        self.layer3 = resnet.layer3
        self.layer4 = resnet.layer4

        self.global_pool = nn.AdaptiveAvgPool2d(1)

        self.global_conv = nn.Sequential(
            nn.Conv2d(2048, 512, 1),
            nn.ReLU(inplace=True),
            nn.Conv2d(512, 256, 1),
            nn.ReLU(inplace=True),
        )

        self.dec4 = DecoderBlock(
            2048 + 1024 + 256,
            512,
        )

        self.dec3 = DecoderBlock(
            512 + 512,
            256,
        )

        self.dec2 = DecoderBlock(
            256 + 256,
            128,
        )

        self.dec1 = DecoderBlock(
            128 + 64,
            64,
        )

        self.dec0 = DecoderBlock(
            64,
            32,
        )

        self.out_conv = nn.Conv2d(
            32,
            2,
            1,
        )

        self.out_act = nn.Tanh()

    def forward(self, L):
        x = L.repeat(
            1,
            3,
            1,
            1,
        )

        x1 = self.relu(
            self.bn1(
                self.conv1(x)
            )
        )

        x2 = self.maxpool(x1)
        x2 = self.layer1(x2)

        x3 = self.layer2(x2)
        x4 = self.layer3(x3)
        x5 = self.layer4(x4)

        global_feat = self.global_pool(x5)
        global_feat = self.global_conv(global_feat)

        global_feat = nn.functional.interpolate(
            global_feat,
            size=x4.shape[2:],
            mode="bilinear",
            align_corners=True,
        )

        skip4 = torch.cat(
            [
                x4,
                global_feat,
            ],
            dim=1,
        )

        d4 = self.dec4(
            x5,
            skip4,
        )

        d3 = self.dec3(
            d4,
            x3,
        )

        d2 = self.dec2(
            d3,
            x2,
        )

        d1 = self.dec1(
            d2,
            x1,
        )

        d0 = self.dec0(d1)

        ab = self.out_act(
            self.out_conv(d0)
        )

        return ab


# =========================================================
# ФУНКЦИИ ОБРАБОТКИ ИЗОБРАЖЕНИЯ
# =========================================================

def pad_to_multiple(arr, multiple=32):
    height, width = arr.shape[:2]

    pad_h = (
        multiple - height % multiple
    ) % multiple

    pad_w = (
        multiple - width % multiple
    ) % multiple

    padded = np.pad(
        arr,
        (
            (0, pad_h),
            (0, pad_w),
        ),
        mode="reflect",
    )

    return padded, height, width


def unpad(tensor_or_array, height, width):
    return tensor_or_array[
        ...,
        :height,
        :width,
    ]


def image_to_L_tensor(image):
    gray_img = image.convert("L")

    gray_rgb = np.asarray(
        gray_img.convert("RGB"),
        dtype=np.float32,
    ) / 255.0

    lab = rgb2lab(gray_rgb)

    lightness = (
        lab[:, :, 0] / 50.0
    ) - 1.0

    padded, height, width = pad_to_multiple(
        lightness,
        multiple=32,
    )

    lightness_tensor = torch.from_numpy(
        padded
    ).float().unsqueeze(0)

    return (
        lightness_tensor,
        height,
        width,
    )


def lab_to_rgb(L_tensor, ab_tensor):
    L = (
        L_tensor.squeeze(0)
        .cpu()
        .numpy()
        + 1.0
    ) * 50.0

    ab = (
        ab_tensor
        .cpu()
        .numpy()
        .transpose(1, 2, 0)
    ) * 128.0

    lab = np.concatenate(
        [
            L[:, :, None],
            ab,
        ],
        axis=2,
    )

    rgb = lab2rgb(lab)

    return np.clip(
        rgb * 255.0,
        0,
        255,
    ).astype(np.uint8)


# =========================================================
# COLORIZER
# =========================================================

class Colorizer:
    def __init__(self, weights_path):
        try:
            import torch_directml

            self.device = torch_directml.device()

            print(
                f"Colorizer использует DirectML: {self.device}"
            )

        except Exception as error:
            print(
                f"DirectML недоступен: {error}"
            )

            self.device = torch.device("cpu")

        self.model = ColorizerNet(
            pretrained=False
        ).to(self.device)

        if not os.path.exists(weights_path):
            raise FileNotFoundError(
                f"Не найдены веса Colorizer: {weights_path}"
            )

        weights = torch.load(
            weights_path,
            map_location="cpu",
        )

        self.model.load_state_dict(
            weights.get(
                "model",
                weights,
            ),
            strict=True,
        )

        self.model.eval()

        print(
            f"Colorizer загружен: {weights_path}"
        )

    def colorize_bytes(self, image_bytes):
        image = Image.open(
            io.BytesIO(image_bytes)
        ).convert("RGB")

        L_tensor, height, width = (
            image_to_L_tensor(image)
        )

        original_L = unpad(
            L_tensor,
            height,
            width,
        )

        model_input = (
            L_tensor
            .unsqueeze(0)
            .to(self.device)
        )

        with torch.no_grad():
            predicted_ab = self.model(
                model_input
            ).cpu()[0]

        predicted_ab = unpad(
            predicted_ab,
            height,
            width,
        )

        rgb = lab_to_rgb(
            original_L,
            predicted_ab,
        )

        result = Image.fromarray(rgb)

        output = io.BytesIO()

        result.save(
            output,
            format="PNG",
        )

        return output.getvalue()