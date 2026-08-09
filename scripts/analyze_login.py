#!/usr/bin/env python3
"""
登录接口分析脚本 - 自动分析 dedao.cn 的登录API
通过监控网络流量来发现微信和手机登录的真实API接口
"""

import requests
from bs4 import BeautifulSoup
import json
import re

# base URLs
BASE_URL = "https://www.dedao.cn"
LOGIN_URL = f"{BASE_URL}/login"

def fetch_page(url):
    """获取页面内容"""
    headers = {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36',
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
        'Accept-Language': 'zh-CN,zh;q=0.8,en-US;q=0.5,en;q=0.3',
        'Accept-Encoding': 'gzip, deflate',
        'Connection': 'keep-alive',
        'Upgrade-Insecure-Requests': '1',
    }
    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        return response.text, response
    except Exception as e:
        print(f"Error fetching {url}: {e}")
        return None, None

def find_login_forms(html):
    """查找页面中的所有登录表单"""
    if not html:
        return []
    
    soup = BeautifulSoup(html, 'html.parser')
    forms = soup.find_all('form')
    
    login_forms = []
    for form in forms:
        action = form.get('action', '')
        method = form.get('method', '').lower()
        inputs = form.find_all('input')
        
        # Check if it's a login form
        has_login_input = any(
            input.get('name', '').lower() in ['username', 'password', 'phone', 'mobile', 'code', 'verify']
            for input in inputs
        )
        has_login_button = any(
            input.get('type', '').lower() == 'submit' or 'login' in str(input).lower()
            for input in inputs
        )
        
        if action or has_login_input or has_login_button:
            login_forms.append({
                'action': action,
                'method': method,
                'inputs': [
                    {'name': input.get('name'), 'type': input.get('type'), 'value': input.get('value', '')}
                    for input in inputs
                ]
            })
    
    return login_forms

def analyze_api_endpoints(html):
    """分析页面中可能存在的API端点"""
    if not html:
        return []
    
    # Find all script tags and extract API endpoints
    soup = BeautifulSoup(html, 'html.parser')
    scripts = soup.find_all('script')
    
    api_patterns = [
        r'/api/(.*?)',
        r"'(https?://[^']+)'",
        r'"(https?://[^"]+)"',
    ]
    
    found_endpoints = set()
    for script in scripts:
        content = str(script.contents[0]) if script.contents else ''
        for pattern in api_patterns:
            matches = re.findall(pattern, content)
            for match in matches:
                if '/api/' in match or 'dedao.cn' in match:
                    found_endpoints.add(match)
    
    return list(found_endpoints)

def main():
    print("=" * 60)
    print("登录接口分析脚本")
    print("=" * 60)
    
    # Step 1: Fetch the login page
    print("\n[1] Fetching login page...")
    html, resp = fetch_page(LOGIN_URL)
    if not html:
        print("Failed to fetch login page")
        return
    
    print(f"Status: {resp.status_code}")
    print(f"Content-Type: {resp.headers.get('Content-Type', '')}")
    
    # Step 2: Find login forms
    print("\n[2] Analyzing login forms...")
    forms = find_login_forms(html)
    if forms:
        print(f"Found {len(forms)} form(s):")
        for i, form in enumerate(forms):
            print(f"\nForm {i+1}:")
            print(f"  Action: {form['action']}")
            print(f"  Method: {form['method']}")
            print(f"  Inputs: {len(form['inputs'])}")
            for inp in form['inputs']:
                print(f"    - {inp['name']} ({inp['type']}): {inp['value'][:50]}...")
    else:
        print("No login forms found (likely uses AJAX instead of traditional forms)")
    
    # Step 3: Analyze API endpoints
    print("\n[3] Analyzing potential API endpoints...")
    endpoints = analyze_api_endpoints(html)
    if endpoints:
        print(f"Found {len(endpoints)} potential API endpoints:")
        for ep in sorted(endpoints)[:30]:  # Limit output
            print(f"  - {ep}")
    else:
        print("No API endpoints found in scripts")
    
    # Step 4: Try to find common login API patterns
    print("\n[4] Searching for common login API patterns...")
    common_patterns = [
        '/login',
        '/api/login',
        '/api/user/login',
        '/auth/login',
        '/oauth/login',
        '/weixin/login',
        '/wechat/login',
        '/sms/login',
        '/mobile/login',
        '/phone/login',
        '/validate/code',
        '/send/code',
        '/verify',
    ]
    
    found_common = []
    for pattern in common_patterns:
        if pattern.lower() in html.lower():
            found_common.append(pattern)
    
    if found_common:
        print(f"Found potential login APIs: {found_common}")
    else:
        print("No common login patterns found in HTML source")
    
    # Print summary
    print("\n" + "=" * 60)
    print("Analysis complete!")
    print(f"HTML length: {len(html)} characters")
    print("=" * 60)

if __name__ == '__main__':
    main()