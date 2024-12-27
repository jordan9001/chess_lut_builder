import matplotlib.pyplot as mpt
import numpy as np
import json



with open("boards.json", "r") as fp:
    data = json.load(fp)

for b in data:
    name = f"{b["Piece"].replace(' ', '')}_{b['Condition'].lower().replace(' ','')}_{b['ConditionValue']}.png"
    print(name)

    board = np.array(b['Board']).reshape(-1, 8)
    mpt.imshow(board, cmap='plasma', interpolation='nearest')
    mpt.colorbar()
    mpt.savefig(name)
    mpt.clf()