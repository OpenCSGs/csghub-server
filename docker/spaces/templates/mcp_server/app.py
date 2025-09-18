import requests
from mcp.server.fastmcp import FastMCP

mcp = FastMCP(name="Echo", host="0.0.0.0", port=8000, log_level="INFO")

@mcp.resource(uri="datasource://sites", description="List supported websites")
def get_sources() -> str:
    """List supported websites"""
    return f"https://www.opencsg.com"

@mcp.tool(name="top_models", description="List the most popular models")
def top_models(num: int) -> str:
    """List the most popular models"""
    resp = requests.get(f"https://hub.opencsg.com/api/v1/models?page=1&per={num}&search=&sort=most_download")
    data = resp.json()
    models = []
    for model in data['data']:
        models.append(f"The model {model['path']} has been downloaded {model['downloads']} times.")
    models_str = ','.join(models)
    return f"{models_str}"

@mcp.prompt(name="get_prompt", description="Create an query prompt")
def get_prompt(num: int) -> str:
    """Create an query prompt"""
    return f"Please find the top {num} models by download count."

if __name__ == "__main__":
    mcp.run(transport='streamable-http')
