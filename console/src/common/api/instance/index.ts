import axios from "axios";

export const instance = axios.create({
    withCredentials: true,
    baseURL: 'http://localhost:9090/api',
})