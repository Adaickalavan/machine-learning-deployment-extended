import unittest
import tensorflow as tf
import os

savedModelPath = os.path.join(os.path.dirname(__file__), "cnn/1/")


def savedModel(path):
    # Recreate the exact same model
    model = tf.keras.experimental.load_from_saved_model(path)
    # model.summary()
    inputLayer = model.input
    outputLayer = model.output
    return inputLayer, outputLayer


class IOTest(unittest.TestCase):
    path = savedModelPath
    inputLayer, outputLayer = savedModel(savedModelPath)

    def testModelInputShape(self):
        self.assertEqual(self.inputLayer.shape.as_list(), [None, 48, 48, 1])

    def testModelInputType(self):
        self.assertEqual(self.inputLayer.dtype, tf.float32)

    def testModelOutputShape(self):
        self.assertEqual(self.outputLayer.shape.as_list(), [None, 7])

    def testModelOutputType(self):
        self.assertEqual(self.outputLayer.dtype, tf.float32)


if __name__ == "__main__":
    unittest.main()
