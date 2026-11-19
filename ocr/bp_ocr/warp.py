import cv2
import numpy as np

from .config import LCD_SIZE


def warp_lcd(image, corners):
    w, h = LCD_SIZE
    target = np.float32([[0, 0], [w - 1, 0], [w - 1, h - 1], [0, h - 1]])
    matrix = cv2.getPerspectiveTransform(corners, target)
    return cv2.warpPerspective(image, matrix, (w, h))
