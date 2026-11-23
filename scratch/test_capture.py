import ctypes
from PIL import Image

u32 = ctypes.windll.user32
gdi32 = ctypes.windll.gdi32

hdesk = u32.OpenDesktopW('Default', 0, False, 0x1FF)
u32.SetThreadDesktop(hdesk)

w = u32.GetSystemMetrics(0)
h = u32.GetSystemMetrics(1)

hdc = u32.GetDC(0)
memdc = gdi32.CreateCompatibleDC(hdc)

class BITMAPINFOHEADER(ctypes.Structure):
    _fields_ = [
        ('biSize', ctypes.c_uint),
        ('biWidth', ctypes.c_int),
        ('biHeight', ctypes.c_int),
        ('biPlanes', ctypes.c_ushort),
        ('biBitCount', ctypes.c_ushort),
        ('biCompression', ctypes.c_uint),
        ('biSizeImage', ctypes.c_uint),
        ('biXPelsPerMeter', ctypes.c_int),
        ('biYPelsPerMeter', ctypes.c_int),
        ('biClrUsed', ctypes.c_uint),
        ('biClrImportant', ctypes.c_uint)
    ]

bmi = BITMAPINFOHEADER()
bmi.biSize = ctypes.sizeof(BITMAPINFOHEADER)
bmi.biWidth = w
bmi.biHeight = -h
bmi.biPlanes = 1
bmi.biBitCount = 32
bmi.biCompression = 0

bits = ctypes.c_void_p()
hbmp = gdi32.CreateDIBSection(memdc, ctypes.byref(bmi), 0, ctypes.byref(bits), 0, 0)
gdi32.SelectObject(memdc, hbmp)
gdi32.BitBlt(memdc, 0, 0, w, h, hdc, 0, 0, 0x00CC0020)

buf = ctypes.string_at(bits.value, w * h * 4)
img = Image.frombytes('RGBA', (w, h), buf, 'raw', 'BGRA')
img.save('scratch/test_real.png')
print('saved test_real.png, size:', img.size)
