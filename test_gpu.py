import torch
import torch_directml

device = torch_directml.device()

print(device)

x = torch.tensor([1, 2, 3])
x = x.to(device)

print(x)
print(x.device)


from basicsr.archs.rrdbnet_arch import RRDBNet

model = RRDBNet(
    num_in_ch=3,
    num_out_ch=3,
    scale=4
)

model = model.to(device)

print("Модель на:", next(model.parameters()).device)