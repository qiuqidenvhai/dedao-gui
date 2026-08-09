#!/usr/bin/env python3
"""
登录API测试脚本 - 通过探测寻找微信和手机登录的真实接口
"""

import requests
import json
import re
from typing import Optional, Dict, Any

BASE_URL = "https://www.dedao.cn"

# Session to maintain cookies
session = requests.Session()
headers = {
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36',
    'Accept': 'application/json, text/javascript, */*; q=0.01',
    'Accept-Language': 'zh-CN,zh;q=0.8,en-US;q=0.5,en;q=0.3',
    'X-Requested-With': 'XMLHttpRequest',
    'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
    'Referer': f"{BASE_URL}/",
}

session.headers.update(headers)

def discover_csrf_token(html: str) -> str:
    patterns = [
        r'"csrfToken"\s*:\s*"([^"]+)"',
        r'csrfToken\s*=\s*"([^"]+)"',
        r'<input[^>]*name[^>]*csrf[^>]*value[^>]*value="([^"]*)"',
    ]
    for pattern in patterns:
        match = re.search(pattern, html)
        if match:
            return match.group(1)
    return ""

def main():
    print("=" * 70)
    print("登录API探索测试")
    print("=" * 70)
    
    # Fetch main page
    print("\n[Step 1] Fetching base URL...")
    try:
        resp = session.get(BASE_URL, timeout=10)
        csrf_token = discover_csrf_token(resp.text)
        print(f"CSRF Token: {csrf_token[:30]}..." if csrf_token else "No CSRF found")
    except Exception as e:
        print(f"Error fetching BASE_URL: {e}")
        return
    
    # Test common dedao API endpoints based on existing code structure
    print("\n[Step 2] Testing dedao API endpoints from source code...")
    
    endpoints_to_test = [
        f"{BASE_URL}/api/search/pc/suggest",
        f"{BASE_URL}/api/hades/v2/product/list",
    ]
    
    for ep in endpoints_to_test:
        print(f"\nTesting POST to {ep}:")
        try:
            test_data = {"query": "test", "searchType": 0}
            post_resp = session.post(ep, data=test_data, timeout=5)
            print(f"  Status: {post_resp.status_code}")
            if post_resp.status_code == 200:
                try:
                    json_data = post_resp.json()
                    print(f"  JSON response keys: {list(json_data.keys()) if isinstance(json_data, dict) else 'N/A'}")
                except:
                    print(f"  Response length: {len(post_resp.text)} chars")
                    print(f"  Preview: {post_resp.text[:200]}")
        except Exception as e:
            print(f"  Error: {e}")

if __name__ == '__main__':
    main()