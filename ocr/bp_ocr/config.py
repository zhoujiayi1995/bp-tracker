import os


def positive_int(name, default):
    value = int(os.environ.get(name, default))
    if value < 1:
        raise ValueError(f"{name} must be positive")
    return value


MAX_EDGE = positive_int("BP_OCR_MAX_EDGE", 1280)
THREADS = positive_int("BP_OCR_THREADS", 1)
LCD_SIZE = (960, 480)

# The numeric libraries read these variables when first imported.
for name in ("OPENBLAS_NUM_THREADS", "OMP_NUM_THREADS", "VECLIB_MAXIMUM_THREADS"):
    os.environ[name] = str(THREADS)

import cv2

cv2.setNumThreads(THREADS)
