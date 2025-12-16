import weave
import json
from datetime import datetime

# Initialize weave
weave.init('mock-trace-example')

# Define a mock LLM call function
@weave.op()
def mock_llm_call(prompt: str) -> str:
    # Instead of making an actual LLM call, we'll just return a mock response
    mock_response = {
        "id": "mock-llm-call-123",
        "model": "gpt-4",
        "created": datetime.now().isoformat(),
        "choices": [{
            "message": {
                "role": "assistant",
                "content": "This is a mock response to: " + prompt
            }
        }]
    }

    return json.dumps(mock_response)

# Create a mock trace
@weave.op()
def create_mock_trace():
    # Call the mock LLM function
    response = mock_llm_call("What is the capital of France?")

    # Return the response
    return {
        "prompt": "What is the capital of France?",
        "response": response,
        "metadata": {
            "model": "gpt-4",
            "temperature": 0.7,
            "max_tokens": 100
        }
    }

if __name__ == "__main__":
    # Create and log the mock trace
    result, call = create_mock_trace.call()

    # Set a custom display name for the call
    call.set_display_name("Mock LLM Call - Capital of France")

    # Add feedback to the call using the correct method
    call.feedback.add("rating", {"value": 5, "comment": "Great response! Very informative."})

    # Get call information and convert datetime objects to ISO format strings
    call_info = {
        "id": call.id,
        "trace_id": call.trace_id,
        "started_at": call.started_at.isoformat() if call.started_at else None,
        "ended_at": call.ended_at.isoformat() if call.ended_at else None,
        "inputs": call.inputs,
        "output": call.output
    }

print("Mock trace created and logged successfully!")
print("Call information:", json.dumps(call_info, indent=2))